package airplay

import (
	"errors"
	"fmt"
	"github.com/davecheney/mdns"
	"log"
)

// Register a DNS Service of name, service type stype, with txt record specified, on
// a certain port. This gets added to a list of registered services which can be
// deregistered by calling ServiceDeregister(). I'm fairly sure that closing the program
// causes the socket connection to mDNSResponder to be closed, which will also deregister
// the services.
func ServiceRegister(name, stype string, txt map[string]string, port uint16) error {
	txtblob, err := mapToBytes(txt)
	if err != nil {
		return err
	}

	entries := []string{
		stype + ".local. 10 IN PTR " + name + "." + stype + ".local.",
		string(append([]byte(name+"."+stype+".local. 10 IN TXT "), txtblob...)),
		fmt.Sprintf(name+"."+stype+".local. 10 IN SRV 0 0 %d arne.local.", port),
		"arne.local. 10 IN AAAA fe80::3608:4ff:fe76:cea5",
		"arne.local. 10 IN A 192.168.1.4",
		"4.1.168.192.in-addr.arpa. 10 IN PTR arne.local.",
		"_services._dns-sd._udp.local. 10 IN PTR " + name + "." + stype + ".local.",
	}
	for _, e := range entries {
		log.Println(e)
		if err := mdns.Publish(e); err != nil {
			log.Println(err)
			return err
		}
	}
	return nil
}

// Deregister all previously allocated services.
func ServiceDeregister() {
}

// Convert a map to the TXT record format: 1 byte (length) followed by
// the character data (string)
func mapToBytes(txt map[string]string) ([]byte, error) {
	blob := make([]byte, 0)
	for key, val := range txt {
		line := "\"" + key + "=" + val + "\" "
		if len(line) > 0xff {
			return nil, errors.New(fmt.Sprintf("Line \"%s\" greater than 255 bytes", line))
		}
		blob = append(blob, []byte(line)...)
	}

	return blob, nil
}
