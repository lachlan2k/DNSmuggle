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
	config Config
	conn   *net.UDPConn
}

func NewFromConfig(config Config) Client {
	return Client{
		config: config,
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

	requestSize := request.GetMaxRequestSize(c.config.TunnelDomain)

	log.Printf("Listening on %v (max datagram size %d)\n", c.config.ListenAddr, requestSize)

	table := NATManager{
		client: c,
	}

	for {
		buff := make([]byte, requestSize)
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
