package networking

import (
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/zap"
)

type mdnsNotifee struct {
	n *P2PNetwork
}

func (n *mdnsNotifee) HandlePeerFound(pi peer.AddrInfo) {
	n.n.Logger.Info("Discovered peer via mDNS", zap.String("peerID", pi.ID.String()))
	if err := n.n.Host.Connect(n.n.Ctx, pi); err != nil {
		n.n.Logger.Error("Failed to connect to peer discovered via mDNS", zap.Error(err))
	} else {
		n.n.addPeer(pi.ID, pi.Addrs)
	}
}
