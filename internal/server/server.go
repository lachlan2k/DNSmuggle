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
	ctrlName := strings.ToUpper(request.GetCtrlFQDN(s.config.TunnelDomain))

	for _, q := range m.Question {
		switch q.Qtype {
		case dns.TypeNS:
			rr, _ := dns.NewRR(fmt.Sprintf("%s NS %s", q.Name, s.config.Nameserver))
			m.Answer = append(m.Answer, rr)

		case dns.TypeTXT:
			// Every request in the control "channel" is encrypted
			// i'm lazy, so writing goes over the control channel right now
			// but it shouldn't
			upperName := strings.ToUpper(q.Name)
			if strings.HasSuffix(upperName, ctrlName) {
				msg := strings.TrimSuffix(upperName, ctrlName)
				log.Printf("Control query (%d chars) %s\n", len(upperName), msg)

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

				decryptedMessage, err := request.DecryptMessage(msgBytes, s.config.PSK)

				if err != nil {
					log.Printf("Decryption error for %s: %v", msg, err)
					rr, _ := dns.NewRR(fmt.Sprintf("%s TXT no", q.Name))
					m.Answer = append(m.Answer, rr)
					continue
				}

				log.Printf("decrypted message: (%d bytes) %d and %s", len(decryptedMessage), decryptedMessage[0], decryptedMessage[1:])

				responseBytes, err := s.manager.handleMessage(decryptedMessage)

				// log.Printf("Hello %s", responseBytes)

				if err != nil {
					log.Printf("Handling error for %s: %v", msg, err)
					rr, _ := dns.NewRR(fmt.Sprintf("%s TXT sad", q.Name))
					m.Answer = append(m.Answer, rr)
					continue
				}

				log.Printf("unencoded response: %s", hex.EncodeToString(responseBytes))

				encodedResponse := request.EncodeResponse(responseBytes)
				rr, _ := dns.NewRR(fmt.Sprintf("%s TXT %s", q.Name, encodedResponse))
				m.Answer = append(m.Answer, rr)
			} else {
				log.Printf("Hmmm, I got a query for %s?\n", q.Name)
			}

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
	dns.HandleFunc(s.config.TunnelDomain, s.handleDnsRequest)
	dnsServer := &dns.Server{Addr: s.config.ListenAddr, Net: "udp"}

	log.Printf("Starting tunnel server (%s) on %s\n", s.config.TunnelDomain, s.config.ListenAddr)
	err = dnsServer.ListenAndServe()

	return
}
