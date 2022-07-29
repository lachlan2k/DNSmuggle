package main

import (
	"flag"
	"log"

	"github.com/lachlan2k/dns-tunnel/internal/client"
)

func main() {
	tunnelDomain := flag.String("domain", "tunnel.local", "Domain to tunnel over")
	listenAddr := flag.String("listenAddr", "127.0.0.1:4321", "Local port to listen on")
	dialAddr := flag.String("dialAddr", "127.0.0.1:51820", "Remote address to dial")
	resolver := flag.String("resolver", "8.8.8.8", "DNS resolver to use")
	psk := flag.String("psk", "hunter2", "Pre-shared key for symmetric encryption")
	threads := flag.Int("threads", 10, "How many reader/writer threads to use")

	flag.Parse()

	c := client.NewFromConfig(client.Config{
		ListenAddr:   *listenAddr,
		DialAddr:     *dialAddr,
		TunnelDomain: *tunnelDomain,
		Resolver:     *resolver,
		PSK:          *psk,
		Threads:      *threads,
	})

	err := c.Run()

	if err != nil {
		log.Fatalf("Couldn't start client: %v", err)
	}
}
