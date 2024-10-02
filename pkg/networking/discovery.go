// networking/discovery.go

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
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/observability"
)

// DiscoveryService encapsulates peer discovery mechanisms.
type DiscoveryService struct {
	ctx            context.Context
	DHT            *dht.IpfsDHT
	Discovery      *routing.RoutingDiscovery
	Ctx            context.Context
	Logger         logger.Logger
	BootstrapPeers []peer.AddrInfo
	Metrics        *DiscoveryMetrics
	Observability  *observability.Observability
}

// NewDiscoveryService creates a new peer discovery service with a DHT and routing discovery mechanism.
// Parameters:
// - ctx: The context for managing cancellation and deadlines.
// - h: The libp2p host.
// - logger: The logger instance for logging events.
// - bootstrapPeers: A list of bootstrap peers to connect to initially.
// - obs: The observability instance for metrics and tracing.
func NewDiscoveryService(ctx context.Context, h host.Host, logger logger.Logger, bootstrapPeers []peer.AddrInfo, obs *observability.Observability) (*DiscoveryService, error) {
	// Initialize Metrics using Observability's Meter
	metrics, err := InitializeDiscoveryMetrics(ctx, obs.Meter)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize discovery metrics")
	}

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
			startTime := time.Now()
			if err := h.Connect(ctx, peerInfo); err != nil {
				logger.Warn("Failed to connect to bootstrap peer", zap.String("peerID", peerInfo.ID.String()), zap.Error(err))
				metrics.RecordBootstrapPeersFailed(ctx, 1)
			} else {
				logger.Info("Connected to bootstrap peer", zap.String("peerID", peerInfo.ID.String()))
				metrics.RecordBootstrapPeersConnected(ctx, 1)
			}
			latency := time.Since(startTime)
			metrics.RecordDiscoveryLatency(ctx, latency)
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
		ctx:            ctx,
		DHT:            dhtInstance,
		Discovery:      routingDiscovery,
		Ctx:            ctx,
		Logger:         logger,
		BootstrapPeers: bootstrapPeers,
		Metrics:        metrics,
		Observability:  obs,
	}, nil
}

// Advertise advertises the service with the given service tag.
// It continuously attempts to advertise until successful, handling specific retry logic.
func (ds *DiscoveryService) Advertise(serviceTag string) error {
	startTime := time.Now()
	for {
		_, err := ds.Discovery.Advertise(ds.Ctx, serviceTag, discovery.TTL(10*time.Minute))
		if err != nil {
			if err.Error() == "failed to find any peer in table" {
				ds.Logger.Debug("No peers in DHT routing table yet. Retrying advertisement...")
				ds.Metrics.RecordPeersAdvertisementFailure(ds.Ctx, 1)
				time.Sleep(5 * time.Second)
				continue
			}
			ds.Metrics.RecordPeersAdvertisementFailure(ds.Ctx, 1)
			return errors.Wrap(err, "failed to advertise service")
		}
		ds.Logger.Info("Service advertised successfully", zap.String("serviceTag", serviceTag))
		ds.Metrics.RecordPeersAdvertised(ds.Ctx, 1)
		latency := time.Since(startTime)
		ds.Metrics.RecordDiscoveryLatency(ds.ctx, latency)
		return nil
	}
}

// FindPeers discovers peers providing the given service.
// It returns a channel through which discovered peers are sent.
func (ds *DiscoveryService) FindPeers(serviceTag string) (<-chan peer.AddrInfo, error) {
	startTime := time.Now()
	peerChan, err := ds.Discovery.FindPeers(ds.Ctx, serviceTag)
	if err != nil {
		ds.Metrics.RecordPeersDiscoveryFailure(ds.Ctx, 1)
		return nil, errors.Wrap(err, "failed to find peers")
	}
	ds.Metrics.RecordPeersDiscovered(ds.Ctx, 1)
	latency := time.Since(startTime)
	ds.Metrics.RecordDiscoveryLatency(ds.Ctx, latency)
	return peerChan, nil
}

// AddBootstrapPeer allows adding new bootstrap peers to the DHT dynamically.
// It connects to the new bootstrap peer and records relevant metrics.
func (ds *DiscoveryService) AddBootstrapPeer(peerInfo peer.AddrInfo) error {
	ds.DHT.Host().Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, peerstore.PermanentAddrTTL)
	startTime := time.Now()
	if err := ds.DHT.Host().Connect(ds.Ctx, peerInfo); err != nil {
		ds.Logger.Warn("Failed to connect to bootstrap peer", zap.String("peerID", peerInfo.ID.String()), zap.Error(err))
		ds.Metrics.RecordBootstrapPeersFailed(ds.Ctx, 1)
		ds.Metrics.RecordDiscoveryLatency(ds.Ctx, time.Since(startTime))
		return errors.Wrapf(err, "failed to connect to bootstrap peer %s", peerInfo.ID.String())
	}
	ds.Logger.Info("Successfully added and connected to new bootstrap peer", zap.String("peerID", peerInfo.ID.String()))
	ds.Metrics.RecordBootstrapPeersConnected(ds.Ctx, 1)
	ds.Metrics.RecordDiscoveryLatency(ds.Ctx, time.Since(startTime))
	return nil
}

// RemovePeer removes a peer from the DHT routing table.
// It updates the relevant metrics to reflect the removal.
func (ds *DiscoveryService) RemovePeer(peerID peer.ID) {
	ds.DHT.Host().Peerstore().ClearAddrs(peerID)
	ds.DHT.RoutingTable().RemovePeer(peerID)
	ds.Logger.Info("Removed peer from routing table", zap.String("peerID", peerID.String()))
	ds.Metrics.RecordPeersRemoved(ds.Ctx, 1)
	ds.Metrics.RecordActivePeers(ds.Ctx, -1) // Decrement active peers by 1
}

// Shutdown gracefully shuts down the discovery service and cleans up resources.
// It ensures that the DHT is closed and any remaining peers are accounted for in metrics.
func (ds *DiscoveryService) Shutdown() error {
	ds.Logger.Info("Shutting down discovery service...")
	if err := ds.DHT.Close(); err != nil {
		// Assuming ListPeers() returns a slice of peer IDs currently in the routing table.
		activePeers := int64(len(ds.DHT.RoutingTable().ListPeers()))
		ds.Metrics.RecordPeersRemoved(ds.Ctx, activePeers)
		ds.Metrics.RecordActivePeers(ds.Ctx, -activePeers)
		return errors.Wrap(err, "failed to shut down DHT")
	}
	return nil
}
