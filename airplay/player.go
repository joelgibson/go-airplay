package airplay

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"flag"
	"fmt"
	mb "github.com/quarnster/util/encoding/binary"
	"io"
	"log"
	"net"
	"os"
	"time"
)

func init() {
	flag.BoolVar(&session_save, "save", session_save, "Whether to save the session or not")
}

type (
	Extension struct {
		Id     uint16
		Length uint16
		Data   []byte `length:"Length*4"`
	}

	RTP struct {
		Version          uint8 `bits:"2"`
		Padding          bool  `bits:"1"`
		ExtensionPresent bool  `bits:"1"`
		CRSCcount        uint8 `bits:"4"`
		Marker           bool  `bits:"1"`
		PayloadType      uint8 `bits:"7"`
		Sequence         uint16
		Timestamp        uint32
		SSRC             uint32
		CRSC             []uint32  `length:"CRSCcount"`
		Extension        Extension `if:"ExtensionPresent"`
	}

	Session struct {
		udpconn       *net.UDPConn
		ctrlconn      *net.UDPConn
		reresend      chan []byte
		ctrladdr      net.Addr
		ctrlseq       uint16
		player        AudioSink
		aesiv, aeskey []byte
		fmtp          []int
		exp_seq       uint16
		bout          io.Writer
	}
)

var (
	udp_port     = 6000
	session_save = false
)

func NewSession(aesiv, aeskey []byte, fmtp []int) (s *Session, err error) {
	s = &Session{aesiv: aesiv, aeskey: aeskey, fmtp: fmtp}
	log.Printf("fmtp: %+v", fmtp)

	udpaddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", udp_port))
	if err != nil {
		return nil, err
	}
	udpaddr2, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", udp_port+1))
	if err != nil {
		return nil, err
	}

	s.reresend = make(chan []byte)
	udp_port += 10
	s.udpconn, err = net.ListenUDP("udp", udpaddr)
	if err != nil {
		return nil, err
	}
	s.ctrlconn, err = net.ListenUDP("udp", udpaddr2)
	if err != nil {
		return nil, err
	}
	go s.ctrlloop()

	s.player, err = CreateAudioSink(s)
	if err != nil {
		return nil, err
	}
	s.bout = bufio.NewWriterSize(s.player, 1420*2)

	go s.loop()
	go func() {
		if err := s.player.Start(); err != nil {
			log.Println("here error")
			log.Println(err)
		}
	}()

	return
}

func (s *Session) Flush() {
	s.player.Flush()
}

func (s *Session) Close() error {
	log.Println("close")
	s.udpconn.Close()
	s.ctrlconn.Close()
	return s.player.Close()
}

func (s *Session) ctrlloop() {
	var (
		f   io.WriteCloser
		err error
	)
	if session_save {
		f, err = os.Create(fmt.Sprintf("session_ctrl_%d.dump", udp_port))
		if err != nil {
			log.Println(err)
			f = nil
		}
		defer f.Close()
	}
	tmp := make([]byte, 4)
	for {
		buf := make([]byte, 4096)
		n, addr, err := s.ctrlconn.ReadFrom(buf) //bin.Read(buf)
		if err != nil {
			log.Println(err)
			return
		}
		if session_save && f != nil {
			binary.LittleEndian.PutUint32(tmp, uint32(n))
			f.Write(tmp[:4])
			f.Write(buf[:n])
		}
		br := mb.BinaryReader{Reader: bytes.NewReader(buf), Endianess: mb.BigEndian}
		var rtp RTP
		br.ReadInterface(&rtp)
		if rtp.PayloadType == 86 {
			s.reresend <- buf[4:n]
		}
		//		log.Printf("ctrl %d %+v", n, rtp)
		//		log.Printf("ctrl %d %+v", n, buf[:n])
		s.ctrladdr = addr
	}
}
func (s *Session) rerequest(start, num uint16) {
	if s.ctrladdr == nil {
		return
	}
	s.ctrlseq++
	data := []byte{0x80, 0x80 | 0x55, 0, 0, 0, 0, 0, 0}
	binary.BigEndian.PutUint16(data[2:], s.ctrlseq)
	binary.BigEndian.PutUint16(data[4:], start)
	binary.BigEndian.PutUint16(data[6:], num)
	log.Printf("rerequesting: %d->%d (%s)", start, start+num, s.ctrladdr.String())
	// br := mb.BinaryReader{Reader: bytes.NewReader(data), Endianess: mb.BigEndian}
	// var rtp RTP
	// log.Println(br.ReadInterface(&rtp))
	// log.Printf("ctrl send %+v", rtp)
	s.ctrlconn.WriteTo(data, s.ctrladdr)
}

func (s *Session) addAudio(packet []byte) error {
	block, err := aes.NewCipher(s.aeskey)
	if err != nil {
		panic(err)
	}
	AESDec := cipher.NewCBCDecrypter(block, s.aesiv)
	//tmp := make([]byte, len(s.aesiv))
	br := mb.BinaryReader{Reader: bytes.NewReader(packet), Endianess: mb.BigEndian}
	var rtp RTP
	br.ReadInterface(&rtp)
	if rtp.Sequence < s.exp_seq {
		return nil
	}
	//log.Printf("addAudio %+v", rtp)
	//seq := binary.BigEndian.Uint16(packet[2:])
	//log.Println(rtp.Sequence, seq)
	if s.exp_seq != 0 && s.exp_seq != rtp.Sequence {
		// Oops, dropped a packet, request retransmission
		s.rerequest(s.exp_seq, rtp.Sequence-s.exp_seq)
		for s.exp_seq < rtp.Sequence {
			log.Printf("Waiting for packet %d", s.exp_seq)
			select {
			case p2 := <-s.reresend:
				s.addAudio(p2)
			case <-time.After(time.Millisecond * 100):
				s.rerequest(s.exp_seq, 1)
			}
		}
		if s.exp_seq > rtp.Sequence {
			// Oops, already processed this one then
			return nil
		}
	}
	s.exp_seq = rtp.Sequence + 1
	audio := packet[12:]
	todec := audio
	//AESDec.CryptBlocks(tmp, s.aesiv)
	for len(todec) >= aes.BlockSize {
		AESDec.CryptBlocks(todec[:aes.BlockSize], todec[:aes.BlockSize])
		todec = todec[aes.BlockSize:]
	}

	_, err = s.bout.Write(audio)
	return err
}

func (s *Session) loop() {
	var (
		f   io.WriteCloser
		err error
	)
	if session_save {
		f, err = os.Create(fmt.Sprintf("session_%d.dump", udp_port))
		if err != nil {
			log.Println(err)
			f = nil
		}
		defer f.Close()
		tmp := make([]byte, 4)
		binary.LittleEndian.PutUint32(tmp, uint32(len(s.fmtp)))
		f.Write(tmp)
		for _, v := range s.fmtp {
			binary.LittleEndian.PutUint32(tmp, uint32(v))
			f.Write(tmp)
		}
		//		log.Println(binary.Write(f, binary.LittleEndian, s.fmtp))
		log.Println(binary.Write(f, binary.LittleEndian, s.aesiv))
		log.Println(binary.Write(f, binary.LittleEndian, s.aeskey))
	}
	buf := make([]byte, 1420)
	bin := s.udpconn //bufio.NewReaderSize(s.udpconn, 128*1024)
	tmp := make([]byte, 4)
	for {
		n, err := bin.Read(buf)
		if err != nil {
			log.Println(err)
			return
		} else if n == 0 {
			// Probably because we set the deadline
			log.Println("deadline reached?")
			return
		}
		packet := buf[:n]
		if session_save && f != nil {
			binary.LittleEndian.PutUint32(tmp, uint32(len(packet)))
			f.Write(tmp[:4])
			binary.Write(f, binary.LittleEndian, packet)
		}

		if err := s.addAudio(packet); err != nil {
			log.Println(err)
			return
		}
	}
}
