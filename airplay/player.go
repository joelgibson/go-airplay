package airplay

import (
	"crypto/aes"
	"crypto/cipher"
	"net"
)

func writeUdp(aesiv, aeskey []byte, fmtp []int) {
	udpaddr, err := net.ResolveUDPAddr("udp", ":6000")
	if err != nil {
		panic(err)
	}
	udpconn, err := net.ListenUDP("udp", udpaddr)
	if err != nil {
		panic(err)
	}
	// Never closes zomg

	packetchan := make(chan []byte, 1000)
	go CreateALACPlayer(fmtp, packetchan)

	buf := make([]byte, 1024*16)
	for {
		n, _, err := udpconn.ReadFromUDP(buf)
		if err != nil {
			panic(err)
		}
		packet := buf[:n]
		audio := packet[12:]
		todec := audio
		block, err := aes.NewCipher(aeskey)
		if err != nil {
			panic(err)
		}
		AESDec := cipher.NewCBCDecrypter(block, aesiv)
		for len(todec) >= aes.BlockSize {
			AESDec.CryptBlocks(todec[:aes.BlockSize], todec[:aes.BlockSize])
			todec = todec[aes.BlockSize:]
		}

		//seqno := int(packet[2])*16 + int(packet[3])
		//if seqno-last_seqno != 1 {
		//	fmt.Println("Blip: last", last_seqno, "next", seqno)
		//}
		send := make([]byte, len(audio))
		copy(send, audio)
		packetchan <- send
	}
}
