package fragmentation

import (
	"bytes"
	"errors"
	"log"
	"reflect"
	"sync"

	"github.com/lachlan2k/dns-tunnel/internal/request"
)

type FragmentationHeader = request.FragmentationHeader

type PacketFragment struct {
	data []byte
	seen bool
}

type Packet struct {
	fragments     []PacketFragment
	receivedCount uint8
	expectedCount uint8
	lock          sync.Mutex
}

func (p *Packet) reset() {
	p.receivedCount = 0
	p.expectedCount = 0
	for i := range p.fragments {
		p.fragments[i] = PacketFragment{
			data: nil,
			seen: false,
		}
	}
}

type FragmentationTable struct {
	packets []Packet
}

func NewFragTable() FragmentationTable {
	packets := make([]Packet, request.MAX_FRAG_ID+1)
	for i := range packets {
		packets[i].fragments = make([]PacketFragment, request.MAX_FRAG_INDEX)
	}

	return FragmentationTable{
		packets: packets,
	}
}

func (t *FragmentationTable) FeedFragment(header FragmentationHeader, data []byte) (completePacket []byte, err error) {
	if header.ID > request.MAX_FRAG_ID {
		err = errors.New("fragmentation header id too large")
		return
	}

	if header.Index > request.MAX_FRAG_INDEX {
		err = errors.New("fragmentation index too large")
		return
	}

	packet := &t.packets[header.ID]
	packet.lock.Lock()
	defer packet.lock.Unlock()

	fragment := &packet.fragments[header.Index]

	// Something is maybe wrong, we've seen this packet fragment before
	// I'll just log this for now to understand when it happens, then i might do some other handling
	if fragment.seen {
		log.Printf("Seen packet fragment %d->%d twice\n", header.ID, header.Index)
		if !reflect.DeepEqual(fragment.data, data) {
			log.Printf("Repeated fragment content differs (%d->%d), resetting packet", header.ID, header.Index)
			packet.reset()
		}
	}

	*fragment = PacketFragment{
		data: data,
		seen: true,
	}

	packet.receivedCount++

	if header.IsFinalFragment {
		packet.expectedCount = header.Index + 1
	}

	if packet.receivedCount == packet.expectedCount {
		// All fragments received, reconstruct the packet
		allFragments := packet.fragments[:packet.receivedCount]
		var buff bytes.Buffer
		for i := range allFragments {
			buff.Write(allFragments[i].data)
		}
		completePacket = buff.Bytes()
		packet.reset()
	}

	return
}
