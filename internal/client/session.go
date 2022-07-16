package client

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math"
	"net"
	"sync"
	"time"

	"github.com/lachlan2k/dns-tunnel/internal/fragmentation"
	"github.com/lachlan2k/dns-tunnel/internal/request"
	"github.com/miekg/dns"
)

type TunnelClientSession struct {
	id            request.SessionID
	client        *Client
	connAddr      *net.UDPAddr
	writeFeed     chan []byte
	fragId        uint16     // TODO: use a pool instead of a counter
	idLock        sync.Mutex // doing some logic to reset it, so atomic operations aren't enough :()
	readFragTable fragmentation.FragmentationTable
}

func newSession(client *Client, conAddr *net.UDPAddr) *TunnelClientSession {
	return &TunnelClientSession{
		client:        client,
		connAddr:      conAddr,
		writeFeed:     make(chan []byte),
		fragId:        0,
		readFragTable: fragmentation.NewFragTable(),
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

		fragmentCount := uint8(math.Ceil(float64(len(datagram)) / float64(sess.client.requestSize)))
		log.Printf("Going to send datagram with %d fragments: %s", fragmentCount, datagram)

		// TODO: use some sort of "pool" instead of a counter
		sess.idLock.Lock()
		id := sess.fragId
		sess.fragId++

		if sess.fragId > request.MAX_FRAG_ID {
			sess.fragId = 0
		}
		sess.idLock.Unlock()

		for i := uint8(0); i < fragmentCount; i++ {
			header := fragmentation.FragmentationHeader{
				Index:           i,
				ID:              id,
				IsFinalFragment: (i + 1) == fragmentCount,
			}

			start := sess.client.requestSize * int(i)
			end := sess.client.requestSize * int(i+1)

			if end > len(datagram) {
				end = len(datagram)
			}

			chunk := datagram[start:end]

			req := request.WriteRequest{
				ID:                  sess.id,
				Data:                chunk,
				FragmentationHeader: header,
			}

			log.Printf("Writing %d->%d: %v", id, i, req)

			sess.sendDataMessage(req.Marshal())
		}

		// log.Printf("Writing datagram %s", datagram)
	}
}

func (sess *TunnelClientSession) readRoutine() {
	sleep := func() {
		fmt.Printf("Poll sleepz zzzz")
		time.Sleep(500 * time.Millisecond)
		fmt.Printf("poll wake")
	}

	for {
		req := request.PollRequest{
			ID: sess.id,
		}

		encodedResponse, err := sess.sendControlChannelMessage(req.Marshal())

		if err != nil {
			sleep()
			continue
		}

		responseBytes, err := request.DecodeResponse(encodedResponse)

		if err != nil {
			log.Printf("Couldn't decode response %s: %v", encodedResponse, err)
			sleep()
			continue
		}

		response, err := request.UnmarshalPollResponse(responseBytes)

		if err != nil {
			log.Printf("Couldn't unmarshal poll resonse %s: %v", responseBytes, err)
			sleep()
			continue
		}

		switch response.Status {
		case request.POLL_NO_DATA:
			log.Printf("Empty poll")
			sleep()
			continue
		case request.POLL_OK:
			log.Printf("Feeding fragment: %v", response)

			completePacket, err := sess.readFragTable.FeedFragment(response.FragmentationHeader, response.Data)

			if err != nil {
				log.Printf("Error feeding fragment (continuing): %v", err)
				continue
			}

			if completePacket != nil {
				_, err = sess.client.conn.WriteToUDP(completePacket, sess.connAddr)
				if err != nil {
					// todo: die
					log.Printf("had writing error (i should probably die now?)")
				}
			}
		}
	}
}

func (sess *TunnelClientSession) sendMessage(msg []byte, controlChannel bool) (response string, err error) {
	var msgBuff bytes.Buffer

	if controlChannel {
		msgBuff.WriteByte(request.REQ_HEADER_CTRL)
		encryptedMsg, err := request.EncryptMessage(msg, sess.client.config.PSK)
		if err != nil {
			return "", err
		}
		msgBuff.Write(encryptedMsg)
	} else {
		msgBuff.WriteByte(request.REQ_HEADER_DATA)
		msgBuff.Write(msg)
	}

	encodedMsg := request.EncodeRequest(msgBuff.Bytes())

	fqdn := dns.Fqdn(encodedMsg + "." + sess.client.config.TunnelDomain)
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

func (sess *TunnelClientSession) sendControlChannelMessage(msg []byte) (response string, err error) {
	return sess.sendMessage(msg, true)
}

func (sess *TunnelClientSession) sendDataMessage(msg []byte) (response string, err error) {
	return sess.sendMessage(msg, false)
}

func (sess *TunnelClientSession) initialise() (err error) {
	req := request.SessionOpenRequest{
		DestAddr: sess.client.config.DialAddr,
	}

	encodedResponse, err := sess.sendControlChannelMessage(req.Marshal())
	if err != nil {
		return
	}

	responseBytes, err := request.DecodeResponse(encodedResponse)
	if err != nil {
		return
	}

	header := responseBytes[0]
	if header == request.SESSION_OPEN_DIAL_FAIL {
		err = errors.New("dial failed on server-side")
		return
	}

	responseBody := responseBytes[1:]
	// todo: responseBytes[0] check if dial error or okay

	log.Printf("Hello our response bytes do be %s", hex.EncodeToString(responseBytes))

	response, err := request.UnmarshalSessionOpenResponse(responseBody)

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
