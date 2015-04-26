package airplay

import (
	"crypto/aes"
	"crypto/cipher"
	//"encoding/binary"
	"bufio"
	"fmt"
	"log"
	"net"
)

type Session struct {
	udpconn       *net.UDPConn
	player        AudioSink
	aesiv, aeskey []byte
	fmtp          []int
}

var udp_port = 6000

func NewSession(aesiv, aeskey []byte, fmtp []int) (s *Session, err error) {
	s = &Session{aesiv: aesiv, aeskey: aeskey, fmtp: fmtp}
	log.Printf("fmtp: %+v", fmtp)
	udpaddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", udp_port))
	if err != nil {
		return nil, err
	}
	udp_port += 10
	s.udpconn, err = net.ListenUDP("udp", udpaddr)
	if err != nil {
		return nil, err
	}

	s.player, err = CreateAudioSink(s)
	if err != nil {
		return nil, err
	}
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

func (s *Session) Close() {
	log.Println("close")
	s.udpconn.Close()
	s.player.Close()
}

type RTP struct {
	Version     uint8 `bits:"2"`
	Padding     bool  `bits:"1"`
	Extension   bool  `bits:"1"`
	CRSCcount   uint8 `bits:"4"`
	Marker      bool  `bits:"1"`
	PayloadType uint8 `bits:"7"`
	Sequence    uint16
	Timestamp   uint32
	SSRC        uint32
	CRSC        []uint32 `length:"CRSCcount"`
}

func (s *Session) loop() {
	buf := make([]byte, 1420)
	bin := s.udpconn //bufio.NewReaderSize(s.udpconn, 128*1024)
	bout := bufio.NewWriterSize(s.player, 44100)

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
		//		sequence := binary.BigEndian.Uint16(packet[2:])
		audio := packet[12:]
		todec := audio
		block, err := aes.NewCipher(s.aeskey)
		if err != nil {
			panic(err)
		}
		AESDec := cipher.NewCBCDecrypter(block, s.aesiv)
		for len(todec) >= aes.BlockSize {
			AESDec.CryptBlocks(todec[:aes.BlockSize], todec[:aes.BlockSize])
			todec = todec[aes.BlockSize:]
		}

		if _, err := bout.Write(audio); err != nil {
			log.Println(err)
			return
		}
	}
}
