package airplay

import (
	"errors"
	"fmt"
	"github.com/davecheney/mdns"
	"regexp"
	//	"github.com/miekg/dns"
	"log"
	"net"
)

// Register a DNS Service of name, service type stype, with txt record specified, on
// a certain port. This gets added to a list of registered services which can be
// deregistered by calling ServiceDeregister(). I'm fairly sure that closing the program
// causes the socket connection to mDNSResponder to be closed, which will also deregister
// the services.
func ServiceRegister(name, stype string, txt map[string]string, iface *net.Interface, port uint16) error {
	txtblob, err := mapToBytes(txt)
	if err != nil {
		return err
	}

	addr, err := iface.Addrs()
	if err != nil {
		return err
	}
	var (
		aa string
		ab string
	)
	re := regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)\.(\d+)`)
	for i := range addr {
		add := addr[i].String()
		if match := re.FindStringSubmatch(add); len(match) == 5 {
			aa = match[4] + "." + match[3] + "." + match[2] + "." + match[1] + ".in-addr.arpa."
			ab = match[0]
			break
		} else {
			log.Println(match, add)
		}
	}
	if len(aa) == 0 {
		return fmt.Errorf("Couldn't parse interface address")
	}
	entries := []string{
		stype + ".local. 10 IN PTR " + name + "." + stype + ".local.",
		string(append([]byte(name+"."+stype+".local. 10 IN TXT "), txtblob...)),
		fmt.Sprintf(name+"."+stype+".local. 10 IN SRV 0 0 %d arne.local.", port),
		"arne.local. 10 IN A " + ab,
		aa + " 10 IN A " + ab,
		"_services._dns-sd._udp.local. 10 IN PTR " + name + "." + stype + ".local.",
	}
	for _, e := range entries {
		log.Println(e)
		if err := mdns.Publish(e); err != nil {
			log.Println(err)
			return err
		}
	}
	// "Announce" service by executing a query
	// c, err := net.Dial("udp", "224.0.0.251:5353")
	// if err != nil {
	// 	return err
	// }
	// defer c.Close()
	// var msg dns.Msg
	// msg.SetQuestion("_raop._tcp.local.", dns.TypePTR)
	// buf, err := msg.Pack()
	// if err != nil {
	// 	return err
	// }
	// log.Println("Write")
	// if _, err = c.Write(buf); err != nil {
	// 	return err
	// }
	// buf = make([]byte, 1500)
	// log.Println("Read")
	// n, err := c.Read(buf)
	// log.Println("Unpack")
	// if err != nil {
	// 	return err
	// } else if err = msg.Unpack(buf[:n]); err != nil {
	// 	return err
	// }
	// log.Printf("Got:\n%s", msg.String())
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
