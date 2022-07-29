package client

import (
	"log"
	"net"

	"github.com/lachlan2k/dns-tunnel/internal/request"
	"github.com/miekg/dns"
)

type Config struct {
	ListenAddr   string
	DialAddr     string
	TunnelDomain string
	Resolver     string
	PSK          string
	Threads      int
}

type Client struct {
	config      Config
	conn        *net.UDPConn
	requestSize int
}

func NewFromConfig(config Config) Client {
	return Client{
		config:      config,
		requestSize: request.GetMaxRequestSize(dns.Fqdn(config.TunnelDomain)),
	}
}

func (c *Client) Run() error {
	addr, err := net.ResolveUDPAddr("udp4", c.config.ListenAddr)
	if err != nil {
		return err
	}

	c.conn, err = net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}

	log.Printf("Listening on %v. Chunk size %d for domain %s\n", c.config.ListenAddr, c.requestSize, c.config.TunnelDomain)

	table := NATManager{
		client: c,
	}

	buff := make([]byte, 65507)

	for {
		n, addr, err := c.conn.ReadFromUDP(buff)

		if err != nil {
			log.Printf("Error reading: %v", err)
			continue
		}

		data := make([]byte, n)
		copy(data, buff[:n])
		// log.Printf("Read %s (%d bytes) from %v", data, n, addr)

		go func() {
			sess := table.UpsertSession(addr)
			sess.Write(data)
		}()
	}
}
