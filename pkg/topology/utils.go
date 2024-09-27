// pkg/topology/utils.go
package topology

import "fmt"

// FormatPeerID formats a peer ID for display.
func FormatPeerID(peerID string) string {
	return fmt.Sprintf("PeerID: %s", peerID)
}
