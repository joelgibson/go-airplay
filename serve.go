package main

import "fmt"
import "net"
import "os"
import "bytes"
import "strings"
import "encoding/pem"
import "crypto/x509"
import "errors"
import "crypto/rsa"
import "encoding/base64"
import "math/big"

func main() {
	// Load up RSA stuff
	p, _ := pem.Decode([]byte(super_secret_key))
	key, err := x509.ParsePKCS1PrivateKey(p.Bytes)
	if err != nil { panic(err) }
	port := ":" + os.Args[1]
	fmt.Println(key.PublicKey.N)

	ln, err := net.Listen("tcp", port)
	if err != nil {
		panic(err)
	}
	defer ln.Close()
	i := 0
	fmt.Println("Online")
	for {
		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}
		go func(conn net.Conn, i int) {
			defer conn.Close()
			buf := make([]byte, 100000)
			n, err := conn.Read(buf)
			if err != nil {
				fmt.Println(i, "Error:", err)
				return
			}
			idx := bytes.Index(buf[:n], []byte("\r\n\r\n"))
			if idx < 0 {
				fmt.Println("Header does not end")
				return
			}
			head := buf[:idx]
			fmt.Println("----Headers:")
			fmt.Println(string(head))
			fmt.Println()
			fmt.Println("----Body:")
			fmt.Println(string(buf[idx:n]))
			fmt.Println()

			lines := bytes.Split(head, []byte("\r\n"))
			headers := make(map[string]string)
			for _, line := range lines[1:] {
				idx := bytes.Index(line, []byte(": "))
				if idx < 0 { continue }
				headers[string(line[:idx])] = string(line[idx+2:])
			}

			chal64 := headers["Apple-Challenge"]
			for len(chal64) % 4 != 0 { chal64 += "=" }
			chal, err := base64.StdEncoding.DecodeString(chal64)
			if err != nil {
				fmt.Println("Could not decode Apple-Challenge")
				return
			}

			// Need our local address
			ipaddr := conn.LocalAddr().String()
			ipbytes := 0
			if ipaddr[0] == '[' {
				ipaddr = ipaddr[1:strings.Index(ipaddr, "%")] 	 // IPv6
				ipbytes = 16
			} else {
				ipaddr = ipaddr[:strings.Index(ipaddr, ":")]
				ipbytes = 4
			}
			ip := net.ParseIP(ipaddr)
			if ipbytes == 4 { 
				ip = ip.To4()
			} else {
				ip = ip.To16()
			}
			chal = append(chal, []byte(ip)...)

			// Need our MAC address
			ifaces, err := net.Interfaces()
			if err != nil { fmt.Println("Interfaces:", err); return }
			for _, iface := range ifaces {
				addrs, err := iface.Addrs()
				if err != nil { fmt.Println(err); continue }
				for _, addr := range addrs {
					if strings.Index(addr.String(), ipaddr) >= 0 {
						haddr := iface.HardwareAddr
						chal = append(chal, haddr[:6]...)
					}
				}
			}

			for len(chal) < 0x20 { chal = append(chal, 0) }
			chal = chal[:0x20]

			// Encrypt
			enc, err := PrivateEncrypt(key, chal)
			if err != nil {
				fmt.Println("Error encrypting:", err)
				return
			}

			response := base64.StdEncoding.EncodeToString(enc)
			for response[len(response)-1] == '=' { response = response[:len(response)-1] }
					
			resp := "RTSP/1.0 200 OK\r\nCSeq: " + headers["CSeq"] + "\r\n"
			resp += "Public: ANNOUNCE, SETUP, RECORD, PAUSE, FLUSH, TEARDOWN, OPTIONS, "
			resp += "GET_PARAMETER, SET_PARAMETER, POST, GET\r\n"
			resp += "Apple-Response: " + response + "\r\n"
			resp += "\r\n"

			fmt.Print("Sending:\n", resp)
			conn.Write([]byte(resp))
		}(conn, i)
		i++
	}

	/*
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			fmt.Println(r)
		})
		http.ListenAndServe(port, nil)
	*/
}

func reverse(s []byte) {
	for i := 0; i < len(s)/2; i++ {
		s[i], s[len(s)-i-1] = s[len(s)-i-1], s[i]
	}
}

const super_secret_key string = `-----BEGIN RSA PRIVATE KEY-----
MIIEpQIBAAKCAQEA59dE8qLieItsH1WgjrcFRKj6eUWqi+bGLOX1HL3U3GhC/j0Qg90u3sG/1CUt
wC5vOYvfDmFI6oSFXi5ELabWJmT2dKHzBJKa3k9ok+8t9ucRqMd6DZHJ2YCCLlDRKSKv6kDqnw4U
wPdpOMXziC/AMj3Z/lUVX1G7WSHCAWKf1zNS1eLvqr+boEjXuBOitnZ/bDzPHrTOZz0Dew0uowxf
/+sG+NCK3eQJVxqcaJ/vEHKIVd2M+5qL71yJQ+87X6oV3eaYvt3zWZYD6z5vYTcrtij2VZ9Zmni/
UAaHqn9JdsBWLUEpVviYnhimNVvYFZeCXg/IdTQ+x4IRdiXNv5hEewIDAQABAoIBAQDl8Axy9XfW
BLmkzkEiqoSwF0PsmVrPzH9KsnwLGH+QZlvjWd8SWYGN7u1507HvhF5N3drJoVU3O14nDY4TFQAa
LlJ9VM35AApXaLyY1ERrN7u9ALKd2LUwYhM7Km539O4yUFYikE2nIPscEsA5ltpxOgUGCY7b7ez5
NtD6nL1ZKauw7aNXmVAvmJTcuPxWmoktF3gDJKK2wxZuNGcJE0uFQEG4Z3BrWP7yoNuSK3dii2jm
lpPHr0O/KnPQtzI3eguhe0TwUem/eYSdyzMyVx/YpwkzwtYL3sR5k0o9rKQLtvLzfAqdBxBurciz
aaA/L0HIgAmOit1GJA2saMxTVPNhAoGBAPfgv1oeZxgxmotiCcMXFEQEWflzhWYTsXrhUIuz5jFu
a39GLS99ZEErhLdrwj8rDDViRVJ5skOp9zFvlYAHs0xh92ji1E7V/ysnKBfsMrPkk5KSKPrnjndM
oPdevWnVkgJ5jxFuNgxkOLMuG9i53B4yMvDTCRiIPMQ++N2iLDaRAoGBAO9v//mU8eVkQaoANf0Z
oMjW8CN4xwWA2cSEIHkd9AfFkftuv8oyLDCG3ZAf0vrhrrtkrfa7ef+AUb69DNggq4mHQAYBp7L+
k5DKzJrKuO0r+R0YbY9pZD1+/g9dVt91d6LQNepUE/yY2PP5CNoFmjedpLHMOPFdVgqDzDFxU8hL
AoGBANDrr7xAJbqBjHVwIzQ4To9pb4BNeqDndk5Qe7fT3+/H1njGaC0/rXE0Qb7q5ySgnsCb3DvA
cJyRM9SJ7OKlGt0FMSdJD5KG0XPIpAVNwgpXXH5MDJg09KHeh0kXo+QA6viFBi21y340NonnEfdf
54PX4ZGS/Xac1UK+pLkBB+zRAoGAf0AY3H3qKS2lMEI4bzEFoHeK3G895pDaK3TFBVmD7fV0Zhov
17fegFPMwOII8MisYm9ZfT2Z0s5Ro3s5rkt+nvLAdfC/PYPKzTLalpGSwomSNYJcB9HNMlmhkGzc
1JnLYT4iyUyx6pcZBmCd8bD0iwY/FzcgNDaUmbX9+XDvRA0CgYEAkE7pIPlE71qvfJQgoA9em0gI
LAuE4Pu13aKiJnfft7hIjbK+5kyb3TysZvoyDnb3HOKvInK7vXbKuU4ISgxB2bB3HcYzQMGsz1qJ
2gG0N5hvJpzwwhbhXqFKA4zaaSrw622wDniAK5MlIE0tIAKKP4yxNGjoD2QYjhBGuhvkWKY=
-----END RSA PRIVATE KEY-----`

// RSA Private signing found here: https://groups.google.com/forum/#!topic/golang-nuts/Vocj33WNhJQ
var (
	ErrInputSize  = errors.New("input size too large")
	ErrEncryption = errors.New("encryption error")
)

func PrivateEncrypt(priv *rsa.PrivateKey, data []byte) (enc []byte, err error) {

	k := (priv.N.BitLen() + 7) / 8
	tLen := len(data)
	// rfc2313, section 8:
	// The length of the data D shall not be more than k-11 octets
	if tLen > k-11 {
		err = ErrInputSize
		return
	}
	em := make([]byte, k)
	em[1] = 1
	for i := 2; i < k-tLen-1; i++ {
		em[i] = 0xff
	}
	copy(em[k-tLen:k], data)
	c := new(big.Int).SetBytes(em)
	if c.Cmp(priv.N) > 0 {
		err = ErrEncryption
		return
	}
	var m *big.Int
	var ir *big.Int
	if priv.Precomputed.Dp == nil {
		m = new(big.Int).Exp(c, priv.D, priv.N)
	} else {
		// We have the precalculated values needed for the CRT.
		m = new(big.Int).Exp(c, priv.Precomputed.Dp, priv.Primes[0])
		m2 := new(big.Int).Exp(c, priv.Precomputed.Dq, priv.Primes[1])
		m.Sub(m, m2)
		if m.Sign() < 0 {
			m.Add(m, priv.Primes[0])
		}
		m.Mul(m, priv.Precomputed.Qinv)
		m.Mod(m, priv.Primes[0])
		m.Mul(m, priv.Primes[1])
		m.Add(m, m2)

		for i, values := range priv.Precomputed.CRTValues {
			prime := priv.Primes[2+i]
			m2.Exp(c, values.Exp, prime)
			m2.Sub(m2, m)
			m2.Mul(m2, values.Coeff)
			m2.Mod(m2, prime)
			if m2.Sign() < 0 {
				m2.Add(m2, prime)
			}
			m2.Mul(m2, values.R)
			m.Add(m, m2)
		}
	}

	if ir != nil {
		// Unblind.
		m.Mul(m, ir)
		m.Mod(m, priv.N)
	}
	enc = m.Bytes()
	return
}
