package config

import (
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
)

// Networking holds the configuration for the P2P networking.
type Networking struct {
	ListenPort     int      `yaml:"listenPort"`
	ProtocolID     string   `yaml:"protocolId"`
	BootstrapPeers []string `yaml:"bootstrapPeers"`
	BootstrapNode  bool     `yaml:"bootstrapNode"` // New field to indicate if the node is a bootstrap node
	EnableMDNS     bool     `yaml:"enable_mdns"`   // New field to enable mDNS

	// New field for the network interface name used by eBPF.
	InterfaceName string `yaml:"interface_name"`
}

// BootstrapPeersAsAddrs converts the BootstrapPeers slice into a slice of peer.AddrInfo objects.
func (n *Networking) BootstrapPeersAsAddrs() ([]peer.AddrInfo, error) {
	var addrInfos []peer.AddrInfo

	for _, peerAddr := range n.BootstrapPeers {
		// Convert the string to a multiaddress
		maddr, err := multiaddr.NewMultiaddr(peerAddr)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse multiaddress: %s", peerAddr)
		}

		// Convert the multiaddress to peer.AddrInfo
		addrInfo, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to convert multiaddress to peer.AddrInfo: %s", peerAddr)
		}

		addrInfos = append(addrInfos, *addrInfo)
	}

	return addrInfos, nil
}
