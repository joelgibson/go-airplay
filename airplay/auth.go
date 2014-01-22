package airplay

import (
	"encoding/base64"
	"net"
	"strings"
)

// appleResponse takes an Apple-Challenge header value, and constructs a response for it.
// To derive the response, the IP and MAC of the connection are needed, so the local address
// has to be passed in.
func appleResponse(challenge string, addr net.Addr) (c64 string, err error) {
	// iTunes seems to not pad things. Let's fix that.
	p64 := challenge
	for len(p64)%4 != 0 { p64 += "=" }
	ptext, err := base64.StdEncoding.DecodeString(p64)
	if err != nil { return }

	ptext = append(ptext, GetIP(addr)...)
	ptext = append(ptext, GetMAC(addr)...)
	for len(ptext) < 0x20 { ptext = append(ptext, 0) }

	ctext := ApplyPrivRSA(ptext)
	c64 = base64.StdEncoding.EncodeToString(ctext)

	// We should respond in kind to iTunes
	if len(c64) != len(challenge) {
		if idx := strings.Index(c64, "="); idx >= 0 {
			c64 = c64[:idx]
		}
	}

	return
}
