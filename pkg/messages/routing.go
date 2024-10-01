package messages

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
)

// RouteEntry represents a routing entry with necessary information.
type RouteEntry struct {
	SrcIP      net.IP
	DstIP      net.IP
	SrcPort    uint16
	DstPort    uint16
	NextHopMAC net.HardwareAddr
	Signature  []byte
}

// Serialize serializes the RouteEntry into bytes for network transmission.
func (re *RouteEntry) Serialize() ([]byte, error) {
	buf := new(bytes.Buffer)

	// Serialize IP addresses (16 bytes each for IPv6 compatibility)
	if _, err := buf.Write(re.SrcIP.To16()); err != nil {
		return nil, fmt.Errorf("failed to serialize SrcIP: %w", err)
	}
	if _, err := buf.Write(re.DstIP.To16()); err != nil {
		return nil, fmt.Errorf("failed to serialize DstIP: %w", err)
	}

	// Serialize ports
	if err := binary.Write(buf, binary.BigEndian, re.SrcPort); err != nil {
		return nil, fmt.Errorf("failed to serialize SrcPort: %w", err)
	}
	if err := binary.Write(buf, binary.BigEndian, re.DstPort); err != nil {
		return nil, fmt.Errorf("failed to serialize DstPort: %w", err)
	}

	// Serialize MAC address
	if _, err := buf.Write(re.NextHopMAC); err != nil {
		return nil, fmt.Errorf("failed to serialize NextHopMAC: %w", err)
	}

	// Serialize signature length and signature
	sigLen := uint16(len(re.Signature))
	if err := binary.Write(buf, binary.BigEndian, sigLen); err != nil {
		return nil, fmt.Errorf("failed to serialize Signature length: %w", err)
	}
	if _, err := buf.Write(re.Signature); err != nil {
		return nil, fmt.Errorf("failed to serialize Signature: %w", err)
	}

	return buf.Bytes(), nil
}

// DeserializeRouteEntry deserializes bytes into a RouteEntry.
func DeserializeRouteEntry(data []byte) (*RouteEntry, error) {
	buf := bytes.NewReader(data)
	re := &RouteEntry{}

	// Deserialize IP addresses
	ipBytes := make([]byte, 16)
	if _, err := buf.Read(ipBytes); err != nil {
		return nil, fmt.Errorf("failed to deserialize SrcIP: %w", err)
	}
	re.SrcIP = net.IP(ipBytes)

	if _, err := buf.Read(ipBytes); err != nil {
		return nil, fmt.Errorf("failed to deserialize DstIP: %w", err)
	}
	re.DstIP = net.IP(ipBytes)

	// Deserialize ports
	if err := binary.Read(buf, binary.BigEndian, &re.SrcPort); err != nil {
		return nil, fmt.Errorf("failed to deserialize SrcPort: %w", err)
	}
	if err := binary.Read(buf, binary.BigEndian, &re.DstPort); err != nil {
		return nil, fmt.Errorf("failed to deserialize DstPort: %w", err)
	}

	// Deserialize MAC address
	macBytes := make([]byte, 6)
	if _, err := buf.Read(macBytes); err != nil {
		return nil, fmt.Errorf("failed to deserialize NextHopMAC: %w", err)
	}
	re.NextHopMAC = net.HardwareAddr(macBytes)

	// Deserialize signature
	var sigLen uint16
	if err := binary.Read(buf, binary.BigEndian, &sigLen); err != nil {
		return nil, fmt.Errorf("failed to deserialize Signature length: %w", err)
	}
	re.Signature = make([]byte, sigLen)
	if _, err := buf.Read(re.Signature); err != nil {
		return nil, fmt.Errorf("failed to deserialize Signature: %w", err)
	}

	return re, nil
}
