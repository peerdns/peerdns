package networking

import (
	"context"
	"fmt"
	"log"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/discovery"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
)

// DiscoveryService encapsulates peer discovery mechanisms.
type DiscoveryService struct {
	DHT            *dht.IpfsDHT
	Discovery      *routing.RoutingDiscovery
	Ctx            context.Context
	Logger         *log.Logger
	BootstrapPeers []peer.AddrInfo
}

// NewDiscoveryService creates a new peer discovery service with a DHT and routing discovery mechanism.
func NewDiscoveryService(ctx context.Context, h host.Host, logger *log.Logger, bootstrapPeers []peer.AddrInfo) (*DiscoveryService, error) {
	// Create a new Kademlia DHT instance with server mode enabled for better peer discovery.
	dht, err := dht.New(ctx, h, dht.Mode(dht.ModeServer))
	if err != nil {
		return nil, fmt.Errorf("failed to create DHT: %w", err)
	}

	// Bootstrap the DHT to join the network.
	if err = dht.Bootstrap(ctx); err != nil {
		return nil, fmt.Errorf("failed to bootstrap DHT: %w", err)
	}

	// Add bootstrap peers to the peerstore.
	for _, peerInfo := range bootstrapPeers {
		h.Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, peerstore.PermanentAddrTTL)
	}

	// Connect to each bootstrap peer.
	for _, peerInfo := range bootstrapPeers {
		if err := h.Connect(ctx, peerInfo); err != nil {
			logger.Printf("Failed to connect to bootstrap peer %s: %v", peerInfo.ID, err)
		} else {
			logger.Printf("Connected to bootstrap peer: %s", peerInfo.ID)
		}
	}

	// Create a routing-based discovery service.
	routingDiscovery := routing.NewRoutingDiscovery(dht)

	return &DiscoveryService{
		DHT:            dht,
		Discovery:      routingDiscovery,
		Ctx:            ctx,
		Logger:         logger,
		BootstrapPeers: bootstrapPeers,
	}, nil
}

// NewDiscoveryServiceWithDHT allows the creation of a DiscoveryService using an existing DHT instance.
func NewDiscoveryServiceWithDHT(ctx context.Context, h host.Host, dht *dht.IpfsDHT, logger *log.Logger) (*DiscoveryService, error) {
	// Create a routing-based discovery service using the provided DHT.
	routingDiscovery := routing.NewRoutingDiscovery(dht)

	return &DiscoveryService{
		DHT:       dht,
		Discovery: routingDiscovery,
		Ctx:       ctx,
		Logger:    logger,
	}, nil
}

// Advertise periodically announces the service to the network with retries.
func (ds *DiscoveryService) Advertise(serviceTag string) error {
	retryInterval := time.Second * 5
	maxRetries := 5

	for i := 0; i < maxRetries; i++ {
		_, err := ds.Discovery.Advertise(ds.Ctx, serviceTag, discovery.TTL(10*time.Minute))
		if err == nil {
			ds.Logger.Printf("Service %s advertised successfully", serviceTag)
			return nil
		}

		ds.Logger.Printf("Failed to advertise service %s, retrying... (%d/%d)", serviceTag, i+1, maxRetries)
		time.Sleep(retryInterval)
	}

	return fmt.Errorf("failed to advertise service %s after %d retries", serviceTag, maxRetries)
}

// FindPeers discovers peers providing the given service with a timeout and retry strategy.
func (ds *DiscoveryService) FindPeers(serviceTag string) (<-chan peer.AddrInfo, error) {
	retryInterval := time.Second * 5
	maxRetries := 5

	var peerChan <-chan peer.AddrInfo
	var err error

	for i := 0; i < maxRetries; i++ {
		peerChan, err = ds.Discovery.FindPeers(ds.Ctx, serviceTag)
		if err == nil {
			ds.Logger.Printf("Successfully found peers for service %s", serviceTag)
			return peerChan, nil
		}

		ds.Logger.Printf("Failed to find peers for service %s, retrying... (%d/%d)", serviceTag, i+1, maxRetries)
		time.Sleep(retryInterval)
	}

	return nil, fmt.Errorf("failed to find peers for service %s after %d retries: %w", serviceTag, maxRetries, err)
}

// AddBootstrapPeer allows adding new bootstrap peers to the DHT dynamically.
func (ds *DiscoveryService) AddBootstrapPeer(peerInfo peer.AddrInfo) error {
	ds.DHT.Host().Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, peerstore.PermanentAddrTTL)
	if err := ds.DHT.Host().Connect(ds.Ctx, peerInfo); err != nil {
		ds.Logger.Printf("Failed to connect to bootstrap peer %s: %v", peerInfo.ID, err)
		return err
	}
	ds.Logger.Printf("Successfully added and connected to new bootstrap peer: %s", peerInfo.ID)
	return nil
}

// RemovePeer removes a peer from the DHT routing table.
func (ds *DiscoveryService) RemovePeer(peerID peer.ID) {
	ds.DHT.Host().Peerstore().ClearAddrs(peerID)
	ds.DHT.RoutingTable().RemovePeer(peerID)
	ds.Logger.Printf("Removed peer %s from routing table", peerID)
}

// Shutdown gracefully shuts down the discovery service and cleans up resources.
func (ds *DiscoveryService) Shutdown() error {
	ds.Logger.Println("Shutting down discovery service...")
	if err := ds.DHT.Close(); err != nil {
		return fmt.Errorf("failed to shut down DHT: %w", err)
	}
	return nil
}
