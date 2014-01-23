package airplay

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"log"
	"net"
	"strings"
)

// Set up the private key, from the super_secret_key at the bottom of this file.
var rsaPrivKey *rsa.PrivateKey
func init() {
	pemblock, _ := pem.Decode([]byte(super_secret_key))
	key, err := x509.ParsePKCS1PrivateKey(pemblock.Bytes)
	if err != nil {
		log.Fatalln("Private key could not be parsed:", err)
	}
	rsaPrivKey = key
}


// Will add in padding on strings lacking it.
func base64pad(s string) string {
	for len(s)%4 != 0 {
		s += "="
	}
	return s
}
// Removes all = signs from the back of a string
func base64unpad(s string) string {
	if idx := strings.Index(s, "="); idx >= 0 {
		s = s[:idx]
	}
	return s
}

// appleResponse takes an Apple-Challenge header value, and constructs a response for it.
// To derive the response, the IP and MAC of the connection are needed, so the local address
// has to be passed in.
func appleResponse(challenge string, addr net.Addr) (c64 string, err error) {
	// iTunes seems to not pad things. Let's fix that.
	p64 := base64pad(challenge)
	ptext, err := base64.StdEncoding.DecodeString(p64)
	if err != nil { return }

	ptext = append(ptext, GetIP(addr)...)
	ptext = append(ptext, GetMAC(addr)...)
	for len(ptext) < 0x20 { ptext = append(ptext, 0) }

	ctext, err := rsa.SignPKCS1v15(nil, rsaPrivKey, crypto.Hash(0), ptext)
	if err != nil {
		return
	}
	c64 = base64.StdEncoding.EncodeToString(ctext)

	// We should respond in kind to iTunes
	if len(p64) != len(challenge) {
		c64 = base64unpad(c64)
	}

	return
}

// Takes the rsaaeskey message (in the ANNOUNCE body) and decrypts it to an
// aes key.
func aeskeyFromRsa(rsaaeskey64 string) (key []byte, err error) {
	s64 := base64pad(rsaaeskey64)
	s, err := base64.StdEncoding.DecodeString(s64)
	if err != nil {
		return
	}
	return rsa.DecryptOAEP(sha1.New(), nil, rsaPrivKey, s, nil)
}

// This is straight out of James Laird's Shairport: https://github.com/abrasive/shairport/
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

