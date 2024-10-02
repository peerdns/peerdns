package ebpf

import (
	"encoding/binary"
	"github.com/peerdns/peerdns/pkg/messages"
	"net"
)

type RouteKey struct {
	SrcIP   uint32
	DstIP   uint32
	SrcPort uint16
	DstPort uint16
	Pad     uint32 // Padding to align to 16 bytes
}

type RouteValue struct {
	NextHopMAC [6]byte
	Pad        uint16 // Padding to align to 8 bytes
}

func constructKey(entry *messages.RouteEntry) RouteKey {
	return RouteKey{
		SrcIP:   ipToUint32(entry.SrcIP),
		DstIP:   ipToUint32(entry.DstIP),
		SrcPort: entry.SrcPort,
		DstPort: entry.DstPort,
		Pad:     0,
	}
}

func constructValue(entry *messages.RouteEntry) RouteValue {
	var mac [6]byte
	copy(mac[:], entry.NextHopMAC)
	return RouteValue{
		NextHopMAC: mac,
		Pad:        0,
	}
}

func ipToUint32(ip net.IP) uint32 {
	ip4 := ip.To4()
	if ip4 == nil {
		return 0
	}
	return binary.BigEndian.Uint32(ip4)
}
