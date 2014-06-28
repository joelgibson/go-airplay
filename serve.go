package main

import (
	"flag"
	"github.com/joelgibson/go-airplay/airplay"
	"log"
	"net"
	"os"
)

func main() {
	debug := flag.Bool("debug", false, "Show debug output.")
	name := flag.String("name", "Swaggatron", "AirTunes name.")
	flag.Parse()

	log.SetFlags(log.Flags() | log.Llongfile)
	if *debug {
		airplay.Debug = log.New(os.Stderr, "DEBUG ", log.LstdFlags)
	}
	err := airplay.ServeAirTunes(*name, func(id string, conn net.Conn) {
		airplay.RtspSession(id, conn, func(x chan string) {})
	})
	if err != nil {
		log.Fatal(err)
	}
}
