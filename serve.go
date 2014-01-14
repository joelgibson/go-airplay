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
)

const OPTS_RAOP string = "ANNOUNCE, SETUP, RECORD, PAUSE, FLUSH, TEARDOWN, OPTIONS, GET_PARAMETER, SET_PARAMETER, POST, GET"

// Globals, because I can
var AESDec cipher.BlockMode

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
			}
			for len(rsaaeskey64)%4 != 0 {
				rsaaeskey64 += "="
			}
			for len(aesiv64)%4 != 0 {
				aesiv64 += "="
			}
			rsaaeskey, _ := base64.StdEncoding.DecodeString(rsaaeskey64)
			aesiv, _ := base64.StdEncoding.DecodeString(aesiv64)
			aeskey := Decrypt(rsaaeskey)
			block, err := aes.NewCipher(aeskey)
			if err != nil {
				panic(err)
			}
			AESDec = cipher.NewCBCDecrypter(block, aesiv)

			resp += "RTSP/1.0 200 OK\r\n"
			resp += "Audio-Jack-Status: connected; type=analog\r\n"
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
