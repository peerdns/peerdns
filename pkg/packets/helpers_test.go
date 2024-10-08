package packets

import (
	"bytes"
	"net"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
)

// MockPeerID creates a mock peer.ID from a string.
// It now allows empty strings to represent zero peer.ID without causing the test to fail.
func MockPeerID(t *testing.T, idStr string) peer.ID {
	return peer.ID(idStr)
}

// Equal checks if two byte slices are equal.
func Equal(a, b []byte) bool {
	return bytes.Equal(a, b)
}

// MockMACAddress creates a mock MAC address.
func MockMACAddress() net.HardwareAddr {
	return net.HardwareAddr{0x00, 0x1A, 0x2B, 0x3C, 0x4D, 0x5E}
}

// MockIPAddress creates a mock IP address (IPv6 for compatibility).
func MockIPAddress(ipStr string) net.IP {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return net.IPv6zero
	}
	return ip.To16()
}
