package airplay

// #cgo CFLAGS: -std=c99
// #cgo LDFLAGS: -framework AudioToolbox
// #include <stdio.h>
// #include <stdlib.h>
// #include <AudioToolbox/AudioToolbox.h>
// #include "core_audio_play.h"
import "C"
import "encoding/binary"
import "fmt"
import "unsafe"

// This constructs the ALAC Magic cookie from the fmtp header
// recieved during RTSP communication. Information available
// http://alac.macosforge.org/trac/browser/trunk/ALACMagicCookieDescription.txt
func magicCookieFromFmtp(fmtp []int) C.ALACMagicCookie{
	var ccookie C.ALACMagicCookie
	cookie := ((*[24]byte)(unsafe.Pointer(&ccookie)))[:];
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

var peeked []byte
var packetsin chan []byte

//export Go_callback
func Go_callback(playerInfo unsafe.Pointer, queue C.AudioQueueRef, buffer C.AudioQueueBufferRef) {
	audioData := ((*[1<<30]byte)(unsafe.Pointer((*buffer).mAudioData)))[:buffer.mAudioDataBytesCapacity]
	pktDescs := (*[1<<30]C.AudioStreamPacketDescription)(unsafe.Pointer(buffer.mPacketDescriptions))
	npackets := 0

	upto := audioData
	if peeked != nil {
		copy(upto, peeked)
		pktDescs[npackets].mStartOffset = C.SInt64(int64(len(audioData) - len(upto)))
		pktDescs[npackets].mDataByteSize = C.UInt32(int32(len(peeked)))
		upto = upto[len(peeked):]
		npackets++
	}

	peeked = <-packetsin
	for len(peeked) <= len(upto) {
		copy(upto, peeked)
		pktDescs[npackets].mStartOffset = C.SInt64(int64(len(audioData) - len(upto)))
		pktDescs[npackets].mDataByteSize = C.UInt32(int32(len(peeked)))
		pktDescs[npackets].mVariableFramesInPacket = C.UInt32(int32(0))
		upto = upto[len(peeked):]
		npackets++
		peeked = <-packetsin
	}

	buffer.mAudioDataByteSize = C.UInt32(len(audioData) - len(upto))
	buffer.mPacketDescriptionCount = C.UInt32(npackets)

	C.AudioQueueEnqueueBuffer(queue, buffer, 0, (*C.AudioStreamPacketDescription)(nil))
	fmt.Println(npackets, "packets eaten")
}


func CreateALACPlayer(fmtp []int, packetsinloc chan []byte) error {
	packetsin = packetsinloc
	cookie := magicCookieFromFmtp(fmtp)
	fmt.Println("Go: Made cookie")
	buflen := uint32(1024*8)
	numbufs := uint32(8);
	numpacks := uint32(20);

	var playerInfo C.PlayerInfo
	caerr := C.setup_queue(cookie, &playerInfo, C.uint32_t(buflen), C.uint32_t(numbufs), C.uint32_t(numpacks))

	if (caerr != 0) {
		fmt.Println("Go: Error:", caerr)
	}

	// Dunno if I should keep this goroutine hanging around?
	select{}

	return nil
}


