// pkg/routing/routing_manager.go

package routing

import (
	"context"
	"fmt"
	"sync"

	"github.com/peerdns/peerdns/pkg/ebpf"
	"github.com/peerdns/peerdns/pkg/logger"
	"github.com/peerdns/peerdns/pkg/networking"
	"go.uber.org/zap"
)

// RoutingManager manages routing entries and interactions with eBPF maps.
type RoutingManager struct {
	EBPFManager     *ebpf.EBPFManager
	IdentityManager *accounts.Manager
	Network         *networking.P2PNetwork
	Routes          map[string]*RouteEntry
	RoutesLock      sync.RWMutex
	Logger          logger.Logger
}

// NewRoutingManager creates a new RoutingManager.
func NewRoutingManager(ebpfManager *ebpf.EBPFManager, identityManager *accounts.Manager, network *networking.P2PNetwork, logger logger.Logger) *RoutingManager {
	return &RoutingManager{
		EBPFManager:     ebpfManager,
		IdentityManager: identityManager,
		Network:         network,
		Routes:          make(map[string]*RouteEntry),
		Logger:          logger,
	}
}

// AddRoute adds a route to the eBPF map and local storage.
func (rm *RoutingManager) AddRoute(entry *RouteEntry) error {
	// Verify signature
	valid, err := entry.VerifyRouteEntrySignature(entry.SrcIP.String(), rm.IdentityManager)
	if err != nil || !valid {
		return fmt.Errorf("invalid signature for route entry")
	}

	// Add to eBPF map
	if err := rm.EBPFManager.AddRoute(entry); err != nil {
		return fmt.Errorf("failed to add route to eBPF: %w", err)
	}

	// Store in local map
	key := rm.getRouteKey(entry)
	rm.RoutesLock.Lock()
	rm.Routes[key] = entry
	rm.RoutesLock.Unlock()

	return nil
}

// DeleteRoute removes a route from the eBPF map and local storage.
func (rm *RoutingManager) DeleteRoute(entry *RouteEntry) error {
	// Remove from eBPF map
	if err := rm.EBPFManager.DeleteRoute(entry); err != nil {
		return fmt.Errorf("failed to delete route from eBPF: %w", err)
	}

	// Remove from local map
	key := rm.getRouteKey(entry)
	rm.RoutesLock.Lock()
	delete(rm.Routes, key)
	rm.RoutesLock.Unlock()

	return nil
}

// ShareRoute shares the route with connected peers.
func (rm *RoutingManager) ShareRoute(entry *RouteEntry) error {
	data, err := entry.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize route entry: %w", err)
	}

	// Broadcast the route entry over PubSub or direct messaging
	err = rm.Network.BroadcastData(context.Background(), data)
	if err != nil {
		return fmt.Errorf("failed to share route entry: %w", err)
	}

	return nil
}

// HandleIncomingRoute processes incoming routing information.
func (rm *RoutingManager) HandleIncomingRoute(data []byte) error {
	entry, err := DeserializeRouteEntry(data)
	if err != nil {
		return fmt.Errorf("failed to deserialize route entry: %w", err)
	}

	// Verify signature
	valid, err := entry.VerifyRouteEntrySignature(entry.SrcIP.String(), rm.IdentityManager)
	if err != nil || !valid {
		return fmt.Errorf("invalid signature for incoming route")
	}

	// Add route
	if err := rm.AddRoute(entry); err != nil {
		return fmt.Errorf("failed to add incoming route: %w", err)
	}

	rm.Logger.Info("Added incoming route", zap.String("src", entry.SrcIP.String()), zap.String("dst", entry.DstIP.String()))
	return nil
}

// getRouteKey generates a unique key for the route.
func (rm *RoutingManager) getRouteKey(entry *RouteEntry) string {
	return fmt.Sprintf("%s:%d-%s:%d", entry.SrcIP.String(), entry.SrcPort, entry.DstIP.String(), entry.DstPort)
}
