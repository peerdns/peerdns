// pkg/metrics/performance_monitor.go

package metrics

import (
	"context"
	"github.com/peerdns/peerdns/pkg/observability"
	"github.com/pkg/errors"
	"strings"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/peerdns/peerdns/pkg/logger"
	"go.uber.org/zap"
)

// PerformanceMonitor pings peers and collects performance metrics.
type PerformanceMonitor struct {
	host           host.Host
	protocolID     protocol.ID
	logger         logger.Logger
	obs            *observability.Observability
	collector      *Collector
	pingInterval   time.Duration
	concurrency    int
	wg             sync.WaitGroup
	ctx            context.Context
	cancel         context.CancelFunc
	metricsTimeout time.Duration
}

// NewPerformanceMonitor initializes a new PerformanceMonitor.
func NewPerformanceMonitor(ctx context.Context, host host.Host, protocolID protocol.ID, logger logger.Logger, obs *observability.Observability, collector *Collector, pingInterval time.Duration, concurrency int, metricsTimeout time.Duration) *PerformanceMonitor {
	ctx, cancel := context.WithCancel(ctx)
	return &PerformanceMonitor{
		host:           host,
		protocolID:     protocolID,
		logger:         logger,
		collector:      collector,
		pingInterval:   pingInterval,
		concurrency:    concurrency,
		ctx:            ctx,
		cancel:         cancel,
		metricsTimeout: metricsTimeout,
	}
}

// Start begins the peer monitoring process.
func (pm *PerformanceMonitor) Start() {
	pm.logger.Info("Starting PerformanceMonitor")
	peerChan := make(chan peer.ID, pm.concurrency)

	// Collect peers to ping
	go func() {
		ticker := time.NewTicker(pm.pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-pm.ctx.Done():
				close(peerChan)
				return
			case <-ticker.C:
				peers := pm.host.Network().Peers()
				pm.logger.Debug("Collected peers to ping", zap.Int("peerCount", len(peers)))
				for _, p := range peers {
					if p == pm.host.ID() {
						continue // Skip self
					}

					pm.logger.Debug("Adding peer to ping channel", zap.String("peerID", p.String()))
					select {
					case peerChan <- p:
					default:
						// Channel is full; skip adding more peers to prevent blocking
						pm.logger.Warn("Peer channel is full; skipping peer", zap.String("peerID", p.String()))
					}
				}
			}
		}
	}()

	// Start worker pool
	for i := 0; i < pm.concurrency; i++ {
		pm.wg.Add(1)
		go pm.worker(peerChan)
	}
}

// worker processes peers from the peer channel.
func (pm *PerformanceMonitor) worker(peerChan <-chan peer.ID) {
	defer pm.wg.Done()
	for {
		select {
		case <-pm.ctx.Done():
			return
		case p, ok := <-peerChan:
			if !ok {
				return
			}
			pm.pingPeer(p)
		}
	}
}

// pingPeer sends a ping to a peer and collects metrics.
func (pm *PerformanceMonitor) pingPeer(p peer.ID) {
	start := time.Now()
	pm.logger.Debug("Pinging peer", zap.String("peerID", p.String()))
	ctx, cancel := context.WithTimeout(pm.ctx, pm.metricsTimeout)
	defer cancel()

	stream, err := pm.host.NewStream(ctx, p, pm.protocolID)
	if err != nil {
		// @TODO: Find better way of dealing with this i/o deadline reached.
		if !errors.Is(err, context.Canceled) && !strings.Contains(err.Error(), "i/o deadline reached") {
			pm.logger.Warn("Failed to create stream for ping",
				zap.String("peerID", p.String()),
				zap.Error(err),
			)
		}
		pm.collector.UpdateResponsiveness(p, 0.0)
		pm.collector.UpdateReliability(p, 0.0)
		return
	}
	defer stream.Close()

	// Send a ping request
	request := []byte("ping")
	_, err = stream.Write(request)
	if err != nil {
		pm.logger.Warn("Failed to send ping",
			zap.String("peerID", p.String()),
			zap.Error(err),
		)
		pm.collector.UpdateResponsiveness(p, 0.0)
		pm.collector.UpdateReliability(p, 0.0)
		return
	}

	// Await pong response
	buf := make([]byte, 1024)
	n, err := stream.Read(buf)
	if err != nil {
		pm.logger.Warn("Failed to read pong",
			zap.String("peerID", p.String()),
			zap.Error(err),
		)
		pm.collector.UpdateResponsiveness(p, 0.0)
		pm.collector.UpdateReliability(p, 0.0)
		return
	}

	response := string(buf[:n])
	pm.logger.Debug("Received response from peer", zap.String("peerID", p.String()), zap.String("response", response))
	if response != "pong" {
		pm.logger.Warn("Invalid pong response",
			zap.String("peerID", p.String()),
			zap.String("response", response),
		)
		pm.collector.UpdateResponsiveness(p, 0.0)
		pm.collector.UpdateReliability(p, 0.0)
		return
	}

	// Calculate latency
	latency := time.Since(start).Seconds()

	// Update utility metrics based on ping success
	pm.collector.UpdateResponsiveness(p, 1.0/latency) // Higher score for lower latency
	pm.collector.UpdateReliability(p, 1.0)            // Full reliability on success

	pm.logger.Info("Received pong from peer", zap.String("peerID", p.String()), zap.Float64("latency", latency))
}

// Stop halts the performance monitoring process gracefully.
func (pm *PerformanceMonitor) Stop() {
	pm.logger.Info("Stopping PerformanceMonitor")
	pm.cancel()
	pm.wg.Wait()
	pm.logger.Info("PerformanceMonitor stopped")
}
