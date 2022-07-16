package server

import (
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/lachlan2k/dns-tunnel/internal/request"
	"github.com/miekg/dns"
)

type Config struct {
	ListenAddr   string
	TunnelDomain string
	Nameserver   string
	PSK          string
}

type Server struct {
	config  Config
	manager SessionManager
}

func NewFromConfig(config Config) Server {
	return Server{
		config: config,
		manager: SessionManager{
			store: make(map[uint64](*Session)),
		},
	}
}

func (s *Server) handleQuery(m *dns.Msg) {
	for _, q := range m.Question {
		switch q.Qtype {
		case dns.TypeNS:
			rr, _ := dns.NewRR(fmt.Sprintf("%s NS %s", q.Name, s.config.Nameserver))
			m.Answer = append(m.Answer, rr)

		case dns.TypeTXT:
			upperName := strings.ToUpper(q.Name)
			msg := strings.TrimSuffix(upperName, strings.ToUpper(s.config.TunnelDomain))

			msgBytes, err := request.DecodeRequest(msg)

			if err == nil && len(msgBytes) == 0 {
				err = errors.New("0 length message received")
			}

			if err != nil {
				log.Printf("Decoding error for %s: %v", msg, err)
				rr, _ := dns.NewRR(fmt.Sprintf("%s TXT no", q.Name))
				m.Answer = append(m.Answer, rr)
				continue
			}

			msgHeader := msgBytes[0]
			msgBody := msgBytes[1:]

			var responseBytes []byte

			switch msgHeader {
			case request.REQ_HEADER_CTRL:
				msgBody, err = request.DecryptMessage(msgBody, s.config.PSK)

				if err != nil {
					log.Printf("Decryption error for %s: %v", msg, err)
					rr, _ := dns.NewRR(fmt.Sprintf("%s TXT no", q.Name))
					m.Answer = append(m.Answer, rr)
					continue
				}

				responseBytes, err = s.manager.handleControlMessage(msgBody)
			case request.REQ_HEADER_DATA:
				log.Printf("Handling data message %s as %s as %s", msg, hex.EncodeToString(msgBytes), msgBody)
				responseBytes, err = s.manager.handleDataMessage(msgBody)
			}

			if err != nil {
				log.Printf("Handling error for %s: %v", msg, err)
				rr, _ := dns.NewRR(fmt.Sprintf("%s TXT sad", q.Name))
				m.Answer = append(m.Answer, rr)
				continue
			}

			encodedResponse := request.EncodeResponse(responseBytes)
			rr, _ := dns.NewRR(fmt.Sprintf("%s TXT %s", q.Name, encodedResponse))
			m.Answer = append(m.Answer, rr)

			fmt.Printf("ans: %s\n", m.Answer)
		}
	}
}

func (s *Server) handleDnsRequest(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Compress = false

	switch r.Opcode {
	case dns.OpcodeQuery:
		s.handleQuery(m)
	}

	w.WriteMsg(m)
}

func (s *Server) Run() (err error) {
	s.config.TunnelDomain = dns.Fqdn(s.config.TunnelDomain)
	dns.HandleFunc(s.config.TunnelDomain, s.handleDnsRequest)
	dnsServer := &dns.Server{Addr: s.config.ListenAddr, Net: "udp"}

	log.Printf("Starting tunnel server (%s) on %s\n", s.config.TunnelDomain, s.config.ListenAddr)
	err = dnsServer.ListenAndServe()

	return
}
