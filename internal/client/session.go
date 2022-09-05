package client

import (
	"bytes"
	"encoding/binary"
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
	writeFeed     chan *request.WriteRequest
	fragId        uint16     // TODO: use a pool instead of a counter
	idLock        sync.Mutex // doing some logic to reset it, so atomic operations aren't enough :()
	readFragTable fragmentation.FragmentationTable
}

func newSession(client *Client, conAddr *net.UDPAddr) *TunnelClientSession {
	return &TunnelClientSession{
		client:        client,
		connAddr:      conAddr,
		writeFeed:     make(chan *request.WriteRequest),
		fragId:        0,
		readFragTable: fragmentation.NewFragTable(),
	}
}

func (sess *TunnelClientSession) Write(datagram []byte) (n int, err error) {
	// Grab a new ID for this packet
	// TODO: use some sort of "pool" instead of a counter
	sess.idLock.Lock()
	id := sess.fragId
	sess.fragId++

	if sess.fragId > request.MAX_FRAG_ID {
		sess.fragId = 0
	}
	sess.idLock.Unlock()

	fragmentCount := uint8(math.Ceil(float64(len(datagram)) / float64(sess.client.requestSize)))

	// Split the packet into several fragments to be reconstructed
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

		req := &request.WriteRequest{
			ID:                  sess.id,
			Data:                chunk,
			FragmentationHeader: header,
		}

		sess.writeFeed <- req
	}

	return len(datagram), nil
}

func (sess *TunnelClientSession) injestPollResponse(response request.PollResponse) (status uint8, err error) {
	status = response.Status

	switch status {
	case request.POLL_OK:
		var completePacket []byte
		completePacket, err = sess.readFragTable.FeedFragment(response.FragmentationHeader, response.Data)

		if err != nil {
			log.Printf("Error feeding fragment (continuing): %v", err)
			return
		}

		if completePacket != nil {
			_, err = sess.client.conn.WriteToUDP(completePacket, sess.connAddr)
			if err != nil {
				// todo: die
				log.Printf("had writing error (i should probably die now?)")
			}
		}

	case request.POLL_ERROR:
		log.Printf("Got a poll error: %v", response)
	}

	return
}

func (sess *TunnelClientSession) writeRoutine() {
	for {
		req := <-sess.writeFeed

		encodedResponse, err := sess.sendDataMessage(req.Marshal())

		if err != nil {
			log.Printf("Error sending write request: %v", err)
			continue
		}

		responseBytes, err := request.DecodeResponse(encodedResponse)

		if err != nil {
			log.Printf("Couldn't decode response %s: %v", encodedResponse, err)
			continue
		}

		response, err := request.UnmarshalPollResponse(responseBytes)

		if err != nil {
			log.Printf("Couldn't unmarshal write response %s: %v", responseBytes, err)
			continue
		}

		sess.injestPollResponse(response)
	}
}

func (sess *TunnelClientSession) readRoutine() {
	sleep := func() {
		time.Sleep(200 * time.Millisecond)
	}

	for {
		req := request.PollRequest{
			ID: sess.id,
		}

		encodedResponse, err := sess.sendControlChannelMessage(req.Marshal())

		if err != nil {
			log.Printf("Error sending data to control channel: %v", err)
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

		status, err := sess.injestPollResponse(response)

		if err != nil {
			log.Printf("Error injesting poll response: %v", err)
			sleep()
			continue
		}

		if status == request.POLL_NO_DATA {
			sleep()
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

	dnsMsg := new(dns.Msg)
	dnsMsg.SetQuestion(fqdn, dns.TypeTXT)

	c := new(dns.Client)
	c.Timeout = 60 * time.Second

	responseMsg, _, err := c.Exchange(dnsMsg, sess.client.config.Resolver)

	if err != nil {
		err = fmt.Errorf("error making dns request (%v) for %s (session %d): %v", err, fqdn, sess.id, responseMsg)
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
	dataToSend := make([]byte, 8+len(msg))
	timestamp := time.Now().UnixMilli()
	binary.BigEndian.PutUint64(dataToSend[0:8], uint64(timestamp))
	copy(dataToSend[8:], msg)

	return sess.sendMessage(dataToSend, true)
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

	response, err := request.UnmarshalSessionOpenResponse(responseBytes)
	if err != nil {
		err = fmt.Errorf("couldn't unmarshal sesion open response: %v", err)
		return
	}

	if response.Status != request.SESSION_OPEN_OK {
		err = fmt.Errorf("session open failed, response: %v", response)
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

	for i := 0; i < sess.client.config.Threads; i++ {
		go sess.writeRoutine()
		go sess.readRoutine()
	}

	return
}
