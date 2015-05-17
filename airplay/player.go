package airplay

import (
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
	"sync"
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
		max_seq       uint16
		depth         uint16
		ctrlf         io.WriteCloser
		ctrlm         sync.Mutex
	}
)

var (
	udp_port     = 6100
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

	s.reresend = make(chan []byte, 100)
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
		err error
	)
	if session_save {
		s.ctrlf, err = os.Create(fmt.Sprintf("session_ctrl_%d.dump", udp_port))
		if err != nil {
			log.Println(err)
			s.ctrlf = nil
		}
		defer s.ctrlf.Close()
	}
	tmp := make([]byte, 4)
	for {
		buf := make([]byte, 4096)
		n, addr, err := s.ctrlconn.ReadFrom(buf) //bin.Read(buf)
		if err != nil {
			log.Println(err)
			return
		}
		if session_save && s.ctrlf != nil {
			binary.LittleEndian.PutUint32(tmp, uint32(n))
			s.ctrlm.Lock()
			s.ctrlf.Write(tmp[:4])
			s.ctrlf.Write(buf[:n])
			s.ctrlm.Unlock()
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
	if session_save && s.ctrlf != nil {
		s.ctrlm.Lock()
		tmp := make([]byte, 4)
		binary.LittleEndian.PutUint32(tmp, uint32(len(data)))
		s.ctrlf.Write(tmp)
		s.ctrlf.Write(data)
		s.ctrlm.Unlock()
	}

	// br := mb.BinaryReader{Reader: bytes.NewReader(data), Endianess: mb.BigEndian}
	// var rtp RTP
	// log.Println(br.ReadInterface(&rtp))
	// log.Printf("ctrl send %+v", rtp)
	s.ctrlconn.WriteTo(data, s.ctrladdr)
}

func (s *Session) addAudio(packet []byte) error {
	s.depth++
	defer func() { s.depth-- }()
	block, err := aes.NewCipher(s.aeskey)
	if err != nil {
		panic(err)
	}
	AESDec := cipher.NewCBCDecrypter(block, s.aesiv)
	//tmp := make([]byte, len(s.aesiv))
	br := mb.BinaryReader{Reader: bytes.NewReader(packet), Endianess: mb.BigEndian}
	var rtp RTP
	br.ReadInterface(&rtp)

	if rtp.Sequence > 0 && s.max_seq-rtp.Sequence > 4096 {
		// Wrapped...
		s.max_seq = rtp.Sequence
		// } else if rtp.Sequence < s.max_seq {
		// 	log.Println("got package", rtp.Sequence)
	}
	if rtp.Sequence == 0 && len(packet) < 12 {
		// TODO: what happens here really?
		rtp.Sequence = binary.BigEndian.Uint16(packet)
		log.Println("Bogus sequence??? 0 ->", rtp.Sequence)
		if s.exp_seq < rtp.Sequence {
			s.exp_seq = rtp.Sequence + 1
			return nil
		}
	}
	if rtp.Sequence < s.exp_seq {
		if !(rtp.Sequence > 0 && s.exp_seq-rtp.Sequence > 4096) {
			// We didn't wrap around, i.e.  we've already processed this Sequence
			return nil
		}
	}
	//log.Printf("addAudio %+v", rtp)
	//seq := binary.BigEndian.Uint16(packet[2:])
	if s.exp_seq != 0 && s.exp_seq < rtp.Sequence && (rtp.Sequence-s.exp_seq) < 50 {
		// Oops, dropped a packet, request retransmission
		//		num := rtp.Sequence - s.exp_seq
		if s.max_seq < rtp.Sequence {
			s.rerequest(s.exp_seq, rtp.Sequence-s.exp_seq)
			s.max_seq = rtp.Sequence
		} else if s.depth == 2 {
			//				num = 1
			s.rerequest(s.exp_seq, 1)
		}
		//	}
		last_exp := s.exp_seq
		reqCnt := 0
	loop:
		for s.exp_seq < rtp.Sequence {
			//			s.rerequest(s.exp_seq, 1)
			//	log.Printf("Have %d, waiting for packet %d", rtp.Sequence, s.exp_seq)
			select {
			case p2 := <-s.reresend:
				s.addAudio(p2)
			case <-time.After(time.Millisecond * 100):
				if s.depth == 1 {
					s.rerequest(s.exp_seq, rtp.Sequence-s.exp_seq)
				} else {
					s.rerequest(s.exp_seq, 1)
				}
				if last_exp == s.exp_seq {
					reqCnt++
					if reqCnt > 10 {
						log.Printf("Request limit reached for %d, giving up", s.exp_seq)
						break loop
					}
				} else {
					reqCnt = 0
				}
				last_exp = s.exp_seq
			}
		}
		if s.exp_seq > rtp.Sequence {
			// Oops, already processed this one then
			return nil
		}
	}
	s.exp_seq = rtp.Sequence + 1
	if len(packet) < 12 {
		return nil
	}
	audio := packet[12:]
	todec := audio
	//AESDec.CryptBlocks(tmp, s.aesiv)
	for len(todec) >= aes.BlockSize {
		AESDec.CryptBlocks(todec[:aes.BlockSize], todec[:aes.BlockSize])
		todec = todec[aes.BlockSize:]
	}

	_, err = s.player.Write(audio)
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
	bin := s.udpconn // bufio.NewReaderSize(s.udpconn, len(buf)*10)
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
