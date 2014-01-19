package main

import (
	"bytes"
	"fmt"
	"net"
	"strings"
	//"encoding/hex"
	. "./airplay"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"os"
	"strconv"
)

const OPTS_RAOP string = "ANNOUNCE, SETUP, RECORD, PAUSE, FLUSH, TEARDOWN, OPTIONS, GET_PARAMETER, SET_PARAMETER, POST, GET"

// Globals, because I can
var aeskey []byte
var aesiv []byte
var fmtp []byte

func writeUdp() {
	f, err := os.Create("data.m4a")
	if err != nil { panic(err) }
	defer f.Close()

	udpaddr, err := net.ResolveUDPAddr("udp", ":6000")
	if err != nil {
		panic(err)
	}
	udpconn, err := net.ListenUDP("udp", udpaddr)
	if err != nil {
		panic(err)
	}

	vlcaddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:1234");
	if err != nil {
		panic(err)
	}
	vlcconn, _ := net.DialUDP("udp", nil, vlcaddr)
	if err != nil {
		panic(err)
	}

	buf := make([]byte, 1024*16)
	last_seqno := 0
	for {
		n, _, err := udpconn.ReadFromUDP(buf)
		if err != nil {
			panic(err)
		}
		packet := buf[:n]
		todec := packet[12:]
		block, err := aes.NewCipher(aeskey)
		if err != nil {
			panic(err)
		}
		AESDec := cipher.NewCBCDecrypter(block, aesiv)
		for len(todec) >= aes.BlockSize {
			AESDec.CryptBlocks(todec[:aes.BlockSize], todec[:aes.BlockSize])
			todec = todec[aes.BlockSize:]
		}

		seqno := int(packet[2])*16 + int(packet[3])
		//tstamp := int(packet[4])<<24 + int(packet[5]) << 16 + int(packet[6]) << 8 + int(packet[7])
		//ssrc := packet[8:8+4]
		audio := packet[8+4:]
		f.Write(audio)
		_, err = vlcconn.Write(packet)
		if err != nil { panic(err) }
		if seqno - last_seqno != 1 { fmt.Println("Blip: last", last_seqno, "next", seqno) }
		last_seqno = seqno
	}
}

func handler(conn net.Conn) {
	defer conn.Close()
	for {
		// Blah boring socket stuff
		buf := make([]byte, 4096)
		n, err := conn.Read(buf)
		if err != nil {
			fmt.Println("Error reading:", err)
			return
		}
		buf = buf[:n]

		// Start deconstructing the HTTP-like block.
		idx := bytes.Index(buf, []byte("\r\n\r\n"))
		if idx < 0 {
			fmt.Println("Header does not end!")
			return
		}

		// Unpack into req (first line), headers, body
		head, body := buf[:idx], buf[idx+4:]
		lines := strings.Split(string(head), "\r\n")
		req := lines[0]
		headers := make(map[string]string)
		for _, line := range lines[1:] {
			idx := strings.Index(line, ": ")
			if idx >= 0 {
				headers[line[:idx]] = line[idx+2:]
			}
		}

		// What method?
		parts := strings.Fields(req)
		if len(req) < 3 {
			fmt.Println("Bad request")
			return
		}

		method := parts[0]
		fmt.Println(method, "Recieved")
		fmt.Printf("====HEADER====\n%s\n====BODY====\n%s\n", head, body)

		resp := ""
		if method == "OPTIONS" {
			resp += "RTSP/1.0 200 OK\r\n"
			resp += "Public: " + OPTS_RAOP + "\r\n"
			resp += "CSeq: " + headers["CSeq"] + "\r\n"
			// Do we have a challenge?
			if challenge, ok := headers["Apple-Challenge"]; ok {
				p64 := challenge
				for len(p64)%4 != 0 {
					p64 += "="
				}
				ptext, _ := base64.StdEncoding.DecodeString(p64)
				ptext = append(ptext, []byte(GetIP(conn))...)
				ptext = append(ptext, []byte(GetMAC(conn)[:6])...)
				for len(ptext) < 0x20 {
					ptext = append(ptext, 0)
				}
				ptext = ptext[:0x20]
				ctext := ApplyPrivRSA(ptext)
				c64 := base64.StdEncoding.EncodeToString(ctext)
				if strings.Index(challenge, "=") < 0 {
					for c64[len(c64)-1] == '=' {
						c64 = c64[:len(c64)-1]
					}
				}
				resp += "Apple-Response: " + c64 + "\r\n"
			}
			resp += "\r\n"
		}

		if method == "ANNOUNCE" {
			lines := strings.Split(string(body), "\r\n")
			fmtphead := "a=fmtp:"
			aeskeyhead := "a=rsaaeskey:"
			aesivhead := "a=aesiv:"
			rsaaeskey64 := ""
			aesiv64 := ""
			for _, line := range lines {
				if idx := strings.Index(line, aeskeyhead); idx == 0 {
					rsaaeskey64 = line[len(aeskeyhead):]
				}
				if idx := strings.Index(line, aesivhead); idx == 0 {
					aesiv64 = line[len(aesivhead):]
				}
				if idx := strings.Index(line, fmtphead); idx == 0 {
					fmtp := make([]int, 12)
					for i, x := range strings.Fields(line[len(fmtphead):]) {
						fmtp[i], err = strconv.Atoi(x)
						if err != nil { panic(err) }
					}
				}
			}
			for len(rsaaeskey64)%4 != 0 {
				rsaaeskey64 += "="
			}
			for len(aesiv64)%4 != 0 {
				aesiv64 += "="
			}
			rsaaeskey, _ := base64.StdEncoding.DecodeString(rsaaeskey64)
			aesiv, _ = base64.StdEncoding.DecodeString(aesiv64)
			aeskey = Decrypt(rsaaeskey)

			resp += "RTSP/1.0 200 OK\r\n"
			//resp += "Audio-Jack-Status: connected; type=analog\r\n"
			resp += "CSeq: " + headers["CSeq"] + "\r\n"
			resp += "\r\n"
		}

		if method == "SETUP" {
			resp += "RTSP/1.0 200 OK\r\n"
			resp += "CSeq: " + headers["CSeq"] + "\r\n"
			resp += "Transport: RTP/AVP/UDP;unicast;interleaved=0-1;mode=record;server_port=6000;control_port=6001;timing_port=6002\r\n"
			resp += "Session: 1\r\n"
			resp += "\r\n"
		}

		if method == "RECORD" {
			resp += "RTSP/1.0 200 OK\r\n"
			resp += "CSeq: " + headers["CSeq"] + "\r\n"
			resp += "Audio-Latency: 2205\r\n"
			resp += "\r\n"

			go writeUdp()
		}

		if method == "SET_PARAMETER" || method == "FLUSH" || method == "TEARDOWN" {
			resp += "RTSP/1.0 200 OK\r\n"
			resp += "CSeq: " + headers["CSeq"] + "\r\n"
			resp += "\r\n"
		}

		fmt.Println("Sending:")
		fmt.Printf("%s", resp)
		conn.Write([]byte(resp))
		//if method == "OPTIONS" { return } // Options seems to not stay open
	}
}

func main() {
	port := ":49152"

	ln, err := net.Listen("tcp", port)
	if err != nil {
		panic(err)
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("Socket error:", err)
			return
		}
		go handler(conn)
	}
}
