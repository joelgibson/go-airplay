package airplay

import (
	"bytes"
	"crypto/aes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
)

// This bundles up a request from the client. Once this has been created, it is
// assumed that it follows the RTSP definition, namely that the CSeq header is
// present.
type RtspRequest struct {
	method string
	uri string
	proto string
	headers map[string]string
	head []byte
	body []byte
	data []byte
}

// This is a response, to be sent from the server to the client.
type RtspResponse struct {
	status int
	headers map[string]string
	data []byte
}

// Renders a response, to be output back.
func (r *RtspResponse) Render() []byte {
	// Quick and dirty for now.
	text := "RTSP/1.0 200 OK\r\n"
	for key, val := range r.headers {
		text += key + ": " + val + "\r\n"
	}
	text += "\r\n"
	text += string(r.data)
	return []byte(text)
}

var headsep = []byte("\r\n\r\n")
var knownMethods = map[string]bool{
	"ANNOUNCE": true, "SETUP": true, "RECORD": true,
  "PAUSE": true, "FLUSH": true, "TEARDOWN": true,
	"OPTIONS": true, "GET_PARAMETER": true, "SET_PARAMETER": true,
  "POST": true, "GET": true,
}

// This is called from the server, and adopts a connection.
func RtspSession(id string, conn net.Conn, playerfn func(chan string)) {
	// The session is persistent, so let's store some per-session stuff here.
	// These should be either nil (indicating have not been filled yet), or
	// their correct lengths (12 for fmtp, aes.BlockSize for the iv, whatever
  // else for the key).
	var fmtp []int
	var aesiv, aeskey []byte

	log.Println(id, "-", conn.RemoteAddr(), "New connection")
	defer func() {
		log.Println(id, "-", "Server closing connection.")
		conn.Close()
	}()

	for {
		req, resp, err := readRtspRequest(conn)
		if err != nil {
			if err == io.EOF {
				log.Println(id, "-", "Client closed connection.")
			} else {
				log.Println(id, "-", "Socket error:", err);
			}
			return
		}

		// Log that thanks
		present := ""
		if _, ok := req.headers["Apple-Challenge"]; ok {
			present = "Apple-Challenge present"
		}
		log.Println(id, "-", req.method, req.uri, req.proto, present)
		Debug.Print("From client:\n", string(req.data))

		// Let's just auto-reply to any challenge that comes our way.
		if challenge, ok := req.headers["Apple-Challenge"]; ok {
			response, err := appleResponse(challenge, conn.LocalAddr())
			if err != nil {
				log.Print("Could not make sense of Apple-Challenge" + challenge)
			} else {
				resp.headers["Apple-Response"] = response
			}
		}

		// The first few of these are listed in the order they generally occur
		// during connection setup.
		switch req.method {
		case "OPTIONS":
			methods := make([]string, 0)
			for method := range knownMethods {
				methods = append(methods, method)
			}
			resp.headers["Public"] = strings.Join(methods, " ")

		case "ANNOUNCE":
			// Pull all of the a=xxx:yyyyy tags from the body
			atags := make(map[string]string)
			for _, line := range strings.Split(string(req.body), "\r\n") {
				start := strings.Index(line, "a=")
				sep := strings.Index(line, ":")
				if start == 0 && start < sep {
					atags[line[2:sep]] = line[sep+1:]
				}
			}
			if fmtpstr, ok := atags["fmtp"]; ok {
				fmtp = make([]int, 0)
				for _, x := range strings.Fields(fmtpstr) {
					i, err := strconv.Atoi(x)
					if err != nil {
						log.Println("Could not convert to int in fmtp:", err)
						return
					}
					fmtp = append(fmtp, i)
				}
				if len(fmtp) != 12 {
					log.Println("fmtp wrong length", err)
					return
				}
			} else {
				log.Println("Did not get fmtp from ANNOUNCE")
				return
			}
			if rsaaeskey64, ok := atags["rsaaeskey"]; ok {
				aeskey, err = aeskeyFromRsa(rsaaeskey64)
				if err != nil {
					log.Println("Error while decoding RSA AES key:", err)
					return
				}
			} else {
				log.Println("No AES key found.")
				return
			}
			if aesiv64, ok := atags["aesiv"]; ok {
				aesiv64 = base64pad(aesiv64)
				aesiv, err = base64.StdEncoding.DecodeString(aesiv64)
				if err != nil {
					log.Println("Error while decoding RSA AES iv:", err)
					return
				}
			} else {
				log.Println("No AES IV found.")
				return
			}
			if len(aesiv) != 0 && len(aesiv) != aes.BlockSize {
				log.Println("AES iv has wrong size.")
				return
			}

		case "SETUP":
			resp.headers["Transport"] = "RTP/AVP/UDP;unicast;interleaved=0-1;mode=record;"
			resp.headers["Transport"] += "server_port=6000;control_port=6001;timing_port=6002"
			resp.headers["Session"] = "1" // This is necessary (why?)
		case "RECORD":
			resp.headers["Audio-Latency"] = "2205"
			go writeUdp(aesiv, aeskey, fmtp)
		case "TEARDOWN":
			resp.headers["Session"] = "1" // Is _this_ necessary?
		case "FLUSH":
			// There is a header like "RTP-Info: seq=25639;rtptime=4037478127". We can probably
			// flush the RTP packets up to there. Try to extract that number.
			seq := -1
			if info, ok := req.headers["RTP-Info"]; ok {
				if idx := strings.Index(info, "seq="); idx >= 0 {
					if no, err := strconv.Atoi(info[idx:strings.Index(info, ";")]); err != nil {
							seq = no
					}
				}
			}

			// Should flush the buffer here?
			Debug.Println("Seq:", seq);

		case "SET_PARAMETER":
			// Volume? Message player.
		default:
			// No-op
		}

		// TODO: Handle errors here?
		response := resp.Render()
		_, err = conn.Write(response)
		if err != nil {
			log.Println(id, "-", "Error while writing response:", err)
			return
		}
		Debug.Print("To client:\n", string(response))
	}
}

// readRtspRequest unpacks an incoming request from a socket, and bundles it
// into an RtspReqest, or errors. It blocks until at least a header (ending in
// "\r\n\r\n") has been read, or the socket has errored. It checks to make sure
// that the method called belongs to the allowed options list, and that CSeq exists.
// It also gives back a partially filled out (CSeq, status code) response.
func readRtspRequest(conn net.Conn) (*RtspRequest, *RtspResponse, error) {
	req, resp := new(RtspRequest), new(RtspResponse)
	buffer := make([]byte, 4096)
	nread := 0

	// Try to keep reading until we have a whole header
	for bytes.Index(buffer[:nread], headsep) < 0 {
		n, err := conn.Read(buffer)
		if err != nil {
			return nil, nil, err
		}
		nread += n
	}

	// Split the header off, and check for prescence of method, uri, protocol.
	req.data = buffer[:nread]
	req.head = buffer[:bytes.Index(buffer[:nread], headsep)]
	req.body = buffer[len(req.head) + len(headsep):]
	lines := strings.Split(string(req.head), "\r\n")
	parts := strings.Fields(lines[0])
	if len(parts) != 3 {
		return nil, nil, errors.New("Method not found")
	}
	req.method, req.uri, req.proto = parts[0], parts[1], parts[2]

	// Assert we know the method
	if _, ok := knownMethods[req.method]; !ok {
		// TODO: Actually, meant to do a 501 Not Implemented
		// http://tools.ietf.org/html/rfc2326#page-30
		return nil, nil, errors.New(fmt.Sprintf("Unknown method %s", req.method))
	}

	// Read in all the request headers
	req.headers = make(map[string]string)
	for _, line := range lines[1:] {
		if idx := strings.Index(line, ": "); idx >= 0 {
			req.headers[line[:idx]] = line[idx+2:]
		}
	}

	// Assert that the mandatory CSeq field is present.
	if _, ok := req.headers["CSeq"]; !ok {
		return nil, nil, errors.New("CSeq not present in packet")
	}

	// Check to see if we have a body we need to read more of.
	bodylen, err := strconv.Atoi(req.headers["Content-Length"])
	if err != nil { bodylen = 0 }

	// At the moment just reject if it's longer than the buffer
	if len(req.body) < bodylen {
		return nil, nil, errors.New(fmt.Sprintf("Body length too great: %d bytes", bodylen))
	}

	// Nothing more to do here.
	resp.status = 200
	resp.headers = map[string]string{"CSeq": req.headers["CSeq"]}
	return req, resp, nil
}
