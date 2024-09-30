package networking

import (
	"context"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	discovery "github.com/libp2p/go-libp2p/core/discovery"
	host "github.com/libp2p/go-libp2p/core/host"
	peer "github.com/libp2p/go-libp2p/core/peer"
	peerstore "github.com/libp2p/go-libp2p/core/peerstore"
	routing "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// DiscoveryService encapsulates peer discovery mechanisms.
type DiscoveryService struct {
	DHT            *dht.IpfsDHT
	Discovery      *routing.RoutingDiscovery
	Ctx            context.Context
	Logger         logger.Logger
	BootstrapPeers []peer.AddrInfo
}

// NewDiscoveryService creates a new peer discovery service with a DHT and routing discovery mechanism.
func NewDiscoveryService(ctx context.Context, h host.Host, logger logger.Logger, bootstrapPeers []peer.AddrInfo) (*DiscoveryService, error) {
	// Create a new Kademlia DHT instance with server mode enabled for better peer discovery.
	dhtInstance, err := dht.New(ctx, h, dht.Mode(dht.ModeServer))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create DHT instance")
	}

	// If bootstrap peers are provided, connect to them.
	if len(bootstrapPeers) > 0 {
		// Add bootstrap peers to the peerstore.
		for _, peerInfo := range bootstrapPeers {
			h.Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, peerstore.PermanentAddrTTL)
		}

		// Connect to each bootstrap peer.
		for _, peerInfo := range bootstrapPeers {
			if err := h.Connect(ctx, peerInfo); err != nil {
				logger.Warn("Failed to connect to bootstrap peer", zap.String("peerID", peerInfo.ID.String()), zap.Error(err))
			} else {
				logger.Info("Connected to bootstrap peer", zap.String("peerID", peerInfo.ID.String()))
			}
		}
	} else {
		// No bootstrap peers provided. Relying on mDNS or direct connections.
		logger.Info("No bootstrap peers provided. Relying on mDNS or direct connections.")
	}

	// Bootstrap the DHT to join the network.
	if err = dhtInstance.Bootstrap(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to bootstrap DHT")
	}

	// Create a routing-based discovery service.
	routingDiscovery := routing.NewRoutingDiscovery(dhtInstance)

	return &DiscoveryService{
		DHT:            dhtInstance,
		Discovery:      routingDiscovery,
		Ctx:            ctx,
		Logger:         logger,
		BootstrapPeers: bootstrapPeers,
	}, nil
}

func (ds *DiscoveryService) Advertise(serviceTag string) error {
	for {
		_, err := ds.Discovery.Advertise(ds.Ctx, serviceTag, discovery.TTL(10*time.Minute))
		if err != nil {
			if err.Error() == "failed to find any peer in table" {
				ds.Logger.Warn("No peers in DHT routing table yet. Retrying advertisement...")
				time.Sleep(5 * time.Second)
				continue
			}
			return errors.Wrap(err, "failed to advertise service")
		}
		ds.Logger.Info("Service advertised successfully", zap.String("serviceTag", serviceTag))
		return nil
	}
}

// FindPeers discovers peers providing the given service.
func (ds *DiscoveryService) FindPeers(serviceTag string) (<-chan peer.AddrInfo, error) {
	peerChan, err := ds.Discovery.FindPeers(ds.Ctx, serviceTag)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find peers")
	}
	return peerChan, nil
}

// AddBootstrapPeer allows adding new bootstrap peers to the DHT dynamically.
func (ds *DiscoveryService) AddBootstrapPeer(peerInfo peer.AddrInfo) error {
	ds.DHT.Host().Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, peerstore.PermanentAddrTTL)
	if err := ds.DHT.Host().Connect(ds.Ctx, peerInfo); err != nil {
		ds.Logger.Warn("Failed to connect to bootstrap peer", zap.String("peerID", peerInfo.ID.String()), zap.Error(err))
		return errors.Wrapf(err, "failed to connect to bootstrap peer %s", peerInfo.ID.String())
	}
	ds.Logger.Info("Successfully added and connected to new bootstrap peer", zap.String("peerID", peerInfo.ID.String()))
	return nil
}

// RemovePeer removes a peer from the DHT routing table.
func (ds *DiscoveryService) RemovePeer(peerID peer.ID) {
	ds.DHT.Host().Peerstore().ClearAddrs(peerID)
	ds.DHT.RoutingTable().RemovePeer(peerID)
	ds.Logger.Info("Removed peer from routing table", zap.String("peerID", peerID.String()))
}

// Shutdown gracefully shuts down the discovery service and cleans up resources.
func (ds *DiscoveryService) Shutdown() error {
	ds.Logger.Info("Shutting down discovery service...")
	if err := ds.DHT.Close(); err != nil {
		return errors.Wrap(err, "failed to shut down DHT")
	}
	return nil
}
