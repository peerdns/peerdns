// pkg/routing/signature.go

package routing

import (
	"fmt"
)

// SignRouteEntry signs the routing entry using the node's identity.
func (re *RouteEntry) SignRouteEntry(identityManager *accounts.Manager) error {
	data := re.getSignableData()
	signature, err := identityManager.SignData(data)
	if err != nil {
		return fmt.Errorf("failed to sign route entry: %w", err)
	}
	re.Signature = signature
	return nil
}

// VerifyRouteEntrySignature verifies the signature of the route entry.
func (re *RouteEntry) VerifyRouteEntrySignature(peerID string, identityManager *accounts.Manager) (bool, error) {
	data := re.getSignableData()
	valid, err := identityManager.VerifySignature(peerID, data, re.Signature)
	if err != nil {
		return false, fmt.Errorf("failed to verify route entry signature: %w", err)
	}
	return valid, nil
}

// getSignableData constructs the data to be signed.
func (re *RouteEntry) getSignableData() []byte {
	data := []byte{}
	data = append(data, re.SrcIP.To16()...)
	data = append(data, re.DstIP.To16()...)
	data = append(data, uint16ToBytes(re.SrcPort)...)
	data = append(data, uint16ToBytes(re.DstPort)...)
	data = append(data, re.NextHopMAC...)
	return data
}

// Helper function to convert uint16 to bytes.
func uint16ToBytes(val uint16) []byte {
	return []byte{byte(val >> 8), byte(val & 0xff)}
}
