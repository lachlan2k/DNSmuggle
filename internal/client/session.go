package client

import (
	"fmt"
	"log"
	"net"
	"time"

	"capnproto.org/go/capnp/v3"
	"github.com/lachlan2k/dns-tunnel/internal/request"
	"github.com/miekg/dns"
)

type TunnelClientSession struct {
	id        request.SessionID
	client    *Client
	connAddr  *net.UDPAddr
	writeFeed chan []byte
}

func newSession(client *Client, conAddr *net.UDPAddr) *TunnelClientSession {
	return &TunnelClientSession{
		client:    client,
		connAddr:  conAddr,
		writeFeed: make(chan []byte),
	}
}

func (sess *TunnelClientSession) Write(data []byte) (n int, err error) {
	log.Printf("feeding datagram %s", data)
	sess.writeFeed <- data
	return len(data), nil
}

func (sess *TunnelClientSession) writeRoutine() {
	for {
		datagram := <-sess.writeFeed

		log.Printf("Writing datagram %s", datagram)

		msg := request.MarshalMessage(request.REQ_HEADER_SESSION_WRITE, request.SessionWriteRequest{
			ID:   sess.id,
			Data: datagram,
		})

		sess.sendControlChannelMessage(msg)
	}
}

func (sess *TunnelClientSession) readRoutine() {
	sleep := func() {
		time.Sleep(500 * time.Millisecond)
	}

	for {
		pollMsg := request.MarshalMessage(request.REQ_HEADER_SESSION_POLL, request.SessionPollRequest{
			ID: sess.id,
		})

		encodedResponse, err := sess.sendControlChannelMessage(pollMsg)

		if err != nil {
			sleep()
			continue
		}

		responseBytes, err := request.DecodeResponse(encodedResponse)

		if len(responseBytes) <= 1 {
			sleep()
			continue
		}

		// todo switch on header byte
		_ = responseBytes[0]
		data := responseBytes[1:]

		if err != nil {
			log.Printf("Couldn't decode response %s: %v", encodedResponse, err)
			sleep()
			continue
		}

		_, err = sess.client.conn.WriteToUDP(data, sess.connAddr)

		if err != nil {
			// todo: die
			log.Printf("had writing error (i should probably die now?)")
		}
	}
}

func (sess *TunnelClientSession) sendControlChannelMessage(msg *capnp.Message) (response string, err error) {
	packed, err := msg.MarshalPacked()
	if err != nil {
		return
	}

	encryptedMsg, err := request.EncryptMessage(packed, sess.client.config.PSK)

	if err != nil {
		return
	}

	encodedMsg := request.EncodeRequest(encryptedMsg)

	fqdn := dns.Fqdn(encodedMsg + "." + request.GetCtrlFQDN(sess.client.config.TunnelDomain))
	log.Printf("Sending %d byte fqdn: %s", len(fqdn), fqdn)

	dnsMsg := new(dns.Msg)
	dnsMsg.SetQuestion(fqdn, dns.TypeTXT)

	c := new(dns.Client)
	c.Timeout = 60 * time.Second

	responseMsg, _, err := c.Exchange(dnsMsg, sess.client.config.Resolver)

	if err != nil {
		err = fmt.Errorf("error making dns request for %s (session %d): %v", fqdn, sess.id, responseMsg)
		return
	}

	if len(responseMsg.Answer) == 0 {
		err = fmt.Errorf("response had no answers for %s", fqdn)
		return
	}

	// todo: multiple answers?
	txtResponse, ok := responseMsg.Answer[0].(*dns.TXT)

	if !ok {
		err = fmt.Errorf("couldn't turn response (%v) into a txt response", responseMsg.Answer[0])
		return
	}

	if len(txtResponse.Txt) == 0 {
		err = fmt.Errorf("empty txt response")
		return
	}

	response = txtResponse.Txt[0]
	return
}

func (sess *TunnelClientSession) initialise() (err error) {
	packet := request.MarshalMessage(request.REQ_HEADER_SESSION_OPEN, request.SessionOpenRequest{
		DestAddr: sess.client.config.DialAddr,
	})

	log.Printf("Sending %d bytes: %d and %s", len(packet), packet[0], packet[1:])

	encodedResponse, err := sess.sendControlChannelMessage(packet)
	if err != nil {
		return
	}

	responseBytes, err := request.DecodeResponse(encodedResponse)
	if err != nil {
		return
	}

	// todo: responseBytes[0] check if dial error or okay

	response, err := request.UnmarshalMessage[request.SessionOpenResponse](responseBytes[1:])
	if err != nil {
		return
	}

	sess.id = response.ID
	log.Printf("Session initialized with ID %d", sess.id)

	return
}

func (sess *TunnelClientSession) Open() (err error) {
	err = sess.initialise()
	if err != nil {
		return
	}

	go sess.writeRoutine()
	go sess.readRoutine()

	return
}
