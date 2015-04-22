package airplay

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"fmt"
	"log"
	"net"
)

type Session struct {
	udpconn       *net.UDPConn
	player        *ALACPlayer
	aesiv, aeskey []byte
	fmtp          []int
}

var udp_port = 6000

func NewSession(aesiv, aeskey []byte, fmtp []int) (s *Session, err error) {
	s = &Session{aesiv: aesiv, aeskey: aeskey, fmtp: fmtp}
	udpaddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", udp_port))
	if err != nil {
		return nil, err
	}
	udp_port += 10
	s.udpconn, err = net.ListenUDP("udp", udpaddr)
	if err != nil {
		return nil, err
	}

	s.player, err = CreateALACPlayer(fmtp)
	if err != nil {
		return nil, err
	}
	go s.loop()
	go func() {
		if err := s.player.Play(); err != nil {
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

func (s *Session) loop() {
	buf := make([]byte, 1024*16)
	for {
		n, _, err := s.udpconn.ReadFromUDP(buf)
		if err != nil {
			log.Println(err)
			return
		} else if n == 0 {
			// Probably because we set the deadline
			log.Println("deadline reached?")
			return
		}
		packet := buf[:n]
		sequence := binary.BigEndian.Uint16(packet[2:])
		_ = sequence
		//		log.Println(sequence)
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

		send := make([]byte, len(audio))
		copy(send, audio)
		if err := s.player.Enqueue(send); err != nil {
			log.Println(err)
			return
		}
	}
}
