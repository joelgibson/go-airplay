package airplay

/*
#cgo LDFLAGS: -framework AudioToolbox
#include <stdio.h>
#include <stdlib.h>
#include <AudioToolbox/AudioToolbox.h>
typedef struct ALACMagicCookie {
  uint32_t  frameLength;
  uint8_t   compatibleVersion;
  uint8_t   bitDepth;
  uint8_t   pb;
  uint8_t   mb;
  uint8_t   kb;
  uint8_t   numChannels;
  uint16_t  maxRun;
  uint32_t  maxFrameBytes;
  uint32_t  avgBitRate;
  uint32_t  sampleRate;
} ALACMagicCookie;

extern void Go_callback();
// Address as reported by Go will point to a go function wrapper rather than
// the "real" address, so need this utility function.
static void* addr() {return Go_callback;}
*/
import "C"

import (
	"encoding/binary"
	"fmt"
	"log"
	"sync"
	"unsafe"
)

const (
	alac_buffers     = 8
	alac_buffer_size = 1024 * 8
	alac_num_packets = 20
)

type ALACPlayer struct {
	queue     C.AudioQueueRef
	peeked    []byte
	packetsin chan []byte
	buffers   [alac_buffers]C.AudioQueueBufferRef
	cookie    C.ALACMagicCookie
	running   bool
	sync.Mutex
}

// This constructs the ALAC Magic cookie from the fmtp header
// recieved during RTSP communication. Information available
// http://alac.macosforge.org/trac/browser/trunk/ALACMagicCookieDescription.txt
func magicCookieFromFmtp(fmtp []int) C.ALACMagicCookie {
	var ccookie C.ALACMagicCookie
	cookie := ((*[24]byte)(unsafe.Pointer(&ccookie)))[:]
	binary.BigEndian.PutUint32(cookie[0:4], uint32(fmtp[1]))
	for i := 0; i < 6; i++ {
		cookie[4+i] = byte(fmtp[2+i])
	}
	binary.BigEndian.PutUint16(cookie[10:12], uint16(fmtp[8]))
	binary.BigEndian.PutUint32(cookie[12:16], uint32(fmtp[9]))
	binary.BigEndian.PutUint32(cookie[16:20], uint32(fmtp[10]))
	binary.BigEndian.PutUint32(cookie[20:24], uint32(fmtp[11]))

	return ccookie
}

//export Go_callback
func Go_callback(userdata unsafe.Pointer, queue C.AudioQueueRef, buffer C.AudioQueueBufferRef) {
	p := (*ALACPlayer)(userdata)
	audioData := ((*[1 << 30]byte)(unsafe.Pointer(buffer.mAudioData)))[:buffer.mAudioDataBytesCapacity]
	pktDescs := (*[1 << 30]C.AudioStreamPacketDescription)(unsafe.Pointer(buffer.mPacketDescriptions))[:buffer.mPacketDescriptionCapacity]
	npackets := 0

	upto := audioData
	if p.peeked == nil {
		p.peeked = <-p.packetsin
	}

	for npackets < len(pktDescs) && len(p.peeked) <= len(upto) {
		copy(upto, p.peeked)
		pktDescs[npackets].mStartOffset = C.SInt64(int64(len(audioData) - len(upto)))
		pktDescs[npackets].mDataByteSize = C.UInt32(int32(len(p.peeked)))
		pktDescs[npackets].mVariableFramesInPacket = C.UInt32(int32(0))
		upto = upto[len(p.peeked):]
		npackets++
		if p.running {
			p.peeked = <-p.packetsin
		}
	}
	buffer.mAudioDataByteSize = C.UInt32(len(audioData) - len(upto))
	buffer.mPacketDescriptionCount = C.UInt32(npackets)

	if buffer.mAudioDataByteSize > 0 {
		if err := togo(C.AudioQueueEnqueueBuffer(queue, buffer, 0, nil)); err != nil {
			log.Println(err)
		}
	}
}

func (p *ALACPlayer) Close() error {
	log.Println("close")
	defer log.Println("close done")
	p.Lock()
	defer p.Unlock()
	p.running = false
	close(p.packetsin)
	if p.queue != nil {
		C.AudioQueueStop(p.queue, 1)
		C.AudioQueueDispose(p.queue, 1)
	}
	return nil
}

func ntohl(n C.uint32_t) uint32 {
	b := *(*[4]byte)(unsafe.Pointer(&n))
	return binary.BigEndian.Uint32(b[:])
}

func togo(err C.OSStatus) error {
	if err == 0 {
		return nil
	}
	return fmt.Errorf("OSError(%d)", err)
}
func (p *ALACPlayer) setup(fmtp []int) error {
	log.Println("setup")
	p.Lock()
	defer p.Unlock()
	p.running = true
	p.cookie = magicCookieFromFmtp(fmtp)

	// Create Audio Queue for ALAC
	var inFormat C.AudioStreamBasicDescription
	inFormat.mSampleRate = C.Float64(ntohl(p.cookie.sampleRate))
	inFormat.mFormatID = C.kAudioFormatAppleLossless
	inFormat.mFramesPerPacket = C.UInt32(ntohl(p.cookie.frameLength))
	inFormat.mChannelsPerFrame = 2 // Stero TODO: get from fmtp?
	if err := togo(C.AudioQueueNewOutput(
		&inFormat,
		(*[0]byte)(unsafe.Pointer(C.addr())),
		unsafe.Pointer(p), // User data
		nil,               // Run on audio queue's thread
		nil,               // Callback run loop's mode
		0,                 // Reserved
		&p.queue)); err != nil {
		return err
	}

	if err := togo(C.AudioQueueSetProperty(p.queue, C.kAudioQueueProperty_MagicCookie, unsafe.Pointer(&p.cookie), C.UInt32(unsafe.Sizeof(p.cookie)))); err != nil {
		return err
	}

	return nil
}

func (p *ALACPlayer) Play() error {
	log.Println("Play")
	defer log.Println("play done")
	// Create input buffers, and enqueue using callback
	for i := range p.buffers {
		if err := togo(C.AudioQueueAllocateBufferWithPacketDescriptions(
			p.queue, alac_buffer_size, alac_num_packets, &p.buffers[i])); err != nil {
			return err
		}
		Go_callback(unsafe.Pointer(p), p.queue, p.buffers[i])
	}
	if err := togo(C.AudioQueueSetParameter(p.queue, C.kAudioQueueParam_Volume, 1.0)); err != nil {
		return err
	} else if err := togo(C.AudioQueuePrime(p.queue, 0, nil)); err != nil {
		return err
	} else if err := togo(C.AudioQueueStart(p.queue, nil)); err != nil {
		return err
	}
	return nil
}

func (p *ALACPlayer) Enqueue(data []byte) error {
	p.Lock()
	defer p.Unlock()
	if p.running {
		p.packetsin <- data
	}
	return nil
}

func CreateALACPlayer(fmtp []int) (*ALACPlayer, error) {
	var p ALACPlayer
	p.packetsin = make(chan []byte, 1000)

	if err := p.setup(fmtp); err != nil {
		p.Close()
		return nil, err
	}

	return &p, nil
}
