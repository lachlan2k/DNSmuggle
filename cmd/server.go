package main

import (
	"flag"
	"log"

	"github.com/lachlan2k/dns-tunnel/internal/server"
)

func main() {
	tunnelDomain := flag.String("domain", "tunnel.local", "Domain to tunnel over")
	listenAddr := flag.String("listenAddr", "127.0.0.1:5432", "Local port to listen on")
	nameserver := flag.String("nameserver", "ns1.tunnel.local", "NS record to respond with")
	psk := flag.String("psk", "hunter2", "Pre-shared key for symmetric encryption")

	flag.Parse()

	s := server.NewFromConfig(server.Config{
		ListenAddr:   *listenAddr,
		TunnelDomain: *tunnelDomain,
		Nameserver:   *nameserver,
		PSK:          *psk,
	})

	err := s.Run()
	if err != nil {
		log.Fatalf("Couldn't start server: %v", err)
	}
}
