package main

import (
	"./airplay"
	"flag"
	"log"
	"net"
	"os"
)

func main() {
	debug := flag.Bool("debug", false, "Show debug output.")
	flag.Parse()

	if *debug {
		airplay.Debug = log.New(os.Stderr, "DEBUG ", log.LstdFlags,)
	}
	err := airplay.ServeAirTunes("Swaggatron", func (id string, conn net.Conn) {
		airplay.RtspSession(id, conn, func (x chan string) {})
	})
	if err != nil {
		log.Fatal(err)
	}
}
