package airplay

import (
	"net"
	"strings"
)

// Get the bytes of an IP address from a net.Conn
func GetIP(conn net.Conn) net.IP {
	straddr := conn.LocalAddr().String()
	host, _, _ := net.SplitHostPort(straddr)
	idx := strings.Index(host, "%")
	if idx >= 0 {
		host = host[:idx]
	}

	ipaddr := net.ParseIP(host)

	ip := ipaddr.To4()
	if ip == nil {
		ip = ipaddr.To16()
	}
	if ip == nil {
		panic("Could not convert IP")
	}
	return ip
}

// Get the bytes of a MAC address from a net.Conn
func GetMAC(conn net.Conn) net.HardwareAddr {
	ifaces, err := net.Interfaces()
	ip := GetIP(conn)
	if err != nil {
		panic("Cannot access interfaces")
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			panic(err)
		}
		for _, addr := range addrs {
			if strings.Index(addr.String(), ip.String()) >= 0 {
				return iface.HardwareAddr
			}
		}
	}
	return nil
}
