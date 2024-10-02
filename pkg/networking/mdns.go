package networking

import (
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/zap"
)

// MdnsNotifier implements the mdns.Notifee interface to handle peer discovery via mDNS.
type MdnsNotifier struct {
	n *Network
}

// HandlePeerFound is called when a new peer is discovered via mDNS.
func (n *MdnsNotifier) HandlePeerFound(pi peer.AddrInfo) {
	n.n.Logger.Info("mDNS discovery found peer",
		zap.String("peerID", pi.ID.String()),
		zap.Strings("addresses", addrStrings(pi.Addrs)),
	)
	if len(pi.Addrs) == 0 {
		n.n.Logger.Warn("Discovered peer has no addresses", zap.String("peerID", pi.ID.String()))
		return
	}
	// Attempt to connect using the AddrInfo directly
	if err := n.n.ConnectPeerInfo(pi); err != nil {
		n.n.Logger.Warn("Failed to connect to discovered peer via mDNS", zap.String("peerID", pi.ID.String()), zap.Error(err))
	} else {
		n.n.Logger.Info("Successfully connected to discovered peer via mDNS", zap.String("peerID", pi.ID.String()))
	}
}
