package airplay

import (
	"encoding/hex"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"strconv"
)

// Debug logger - main can reach in an enable this if it wants
var Debug = log.New(ioutil.Discard, "DEBUG ", log.LstdFlags)

// Default TXT Record, have not come up with a new one yet.
var txt map[string]string = map[string]string{
	"txtvers": "1",
	"pw":      "false",
	"tp":      "UDP",
	"sm":      "false",
	"ek":      "1",
	"cn":      "0,1",
	"ch":      "2",
	"ss":      "16",
	"sr":      "44100",
	"vn":      "3",
	"et":      "0,1",
}

var iface *net.Interface

// ServeAirtunes will start advertising an RAOP service, and start listening for
// incoming connections, calling the player in a new goroutine when appropriate.
func ServeAirTunes(name string, handler func(string, net.Conn)) error {
	address := ":49153"
	ifacename := "en2"

	// Try to grab publish information
	_, portstr, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	port, err := strconv.Atoi(portstr)
	if err != nil {
		return err
	}
	iface, err = net.InterfaceByName(ifacename)
	if err != nil {
		return err
	}

	// Publish the service
	raopName := hex.EncodeToString(iface.HardwareAddr) + "\\@" + name
	err = ServiceRegister(raopName, "_raop._tcp", txt, uint16(port))
	if err != nil {
		return err
	}
	defer ServiceDeregister()
	log.Println("Service", raopName, "registered on address", address)

	// Bind the port
	ln, err := net.Listen("tcp4", address)
	if err != nil {
		return err
	}
	defer ln.Close()
	log.Println("Listening for connections on address", address)

	// Listen for incoming
	for {
		id := strconv.Itoa(rand.Int())
		conn, err := ln.Accept()
		log.Println("accepted connection!")
		if err != nil {
			return err
		}
		go handler(id, conn)
	}
}
