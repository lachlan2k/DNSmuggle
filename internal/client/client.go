package client

import (
	"log"
	"net"

	"github.com/lachlan2k/dns-tunnel/internal/request"
)

type Config struct {
	ListenAddr   string
	DialAddr     string
	TunnelDomain string
	Resolver     string
	PSK          string
}

type Client struct {
	config      Config
	conn        *net.UDPConn
	requestSize int
}

func NewFromConfig(config Config) Client {
	return Client{
		config:      config,
		requestSize: request.GetMaxRequestSize(config.TunnelDomain),
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

	log.Printf("Listening on %v\n", c.config.ListenAddr)

	table := NATManager{
		client: c,
	}

	for {
		buff := make([]byte, 65535)
		n, addr, err := c.conn.ReadFromUDP(buff)

		if err != nil {
			log.Printf("Error reading: %v", err)
			continue
		}

		data := buff[:n]
		log.Printf("Read %s (%d bytes) from %v", data, n, addr)

		go (func() {
			sess := table.UpsertSession(addr)
			sess.Write(data)
		})()
	}
}
