// pkg/metrics/performance_monitor.go

package metrics

import (
	"context"
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
	Host             host.Host
	ProtocolID       protocol.ID
	Logger           logger.Logger
	MetricsCollector *MetricsCollector
	PingInterval     time.Duration
	Concurrency      int
	wg               sync.WaitGroup
	ctx              context.Context
	cancel           context.CancelFunc
	metricsTimeout   time.Duration
}

// NewPerformanceMonitor initializes a new PerformanceMonitor.
func NewPerformanceMonitor(ctx context.Context, host host.Host, protocolID protocol.ID, logger logger.Logger, metricsCollector *MetricsCollector, pingInterval time.Duration, concurrency int, metricsTimeout time.Duration) *PerformanceMonitor {
	ctx, cancel := context.WithCancel(ctx)
	return &PerformanceMonitor{
		Host:             host,
		ProtocolID:       protocolID,
		Logger:           logger,
		MetricsCollector: metricsCollector,
		PingInterval:     pingInterval,
		Concurrency:      concurrency,
		ctx:              ctx,
		cancel:           cancel,
		metricsTimeout:   metricsTimeout,
	}
}

// Start begins the peer monitoring process.
func (pm *PerformanceMonitor) Start() {
	pm.Logger.Info("Starting PerformanceMonitor")
	peerChan := make(chan peer.ID, pm.Concurrency)

	// Collect peers to ping
	go func() {
		ticker := time.NewTicker(pm.PingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-pm.ctx.Done():
				close(peerChan)
				return
			case <-ticker.C:
				peers := pm.Host.Network().Peers()
				pm.Logger.Debug("Collected peers to ping", zap.Int("peerCount", len(peers)))
				for _, p := range peers {
					if p == pm.Host.ID() {
						continue // Skip self
					}
					pm.Logger.Debug("Adding peer to ping channel", zap.String("peerID", p.String()))
					select {
					case peerChan <- p:
					default:
						// Channel is full; skip adding more peers to prevent blocking
						pm.Logger.Warn("Peer channel is full; skipping peer", zap.String("peerID", p.String()))
					}
				}
			}
		}
	}()

	// Start worker pool
	for i := 0; i < pm.Concurrency; i++ {
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
	pm.Logger.Debug("Pinging peer", zap.String("peerID", p.String()))
	ctx, cancel := context.WithTimeout(pm.ctx, pm.metricsTimeout)
	defer cancel()

	stream, err := pm.Host.NewStream(ctx, p, pm.ProtocolID)
	if err != nil {
		pm.Logger.Warn("Failed to create stream for ping",
			zap.String("peerID", p.String()),
			zap.Error(err),
		)
		pm.MetricsCollector.UpdateResponsiveness(p, 0.0)
		pm.MetricsCollector.UpdateReliability(p, 0.0)
		return
	}
	defer stream.Close()

	// Send a ping request
	request := []byte("ping")
	_, err = stream.Write(request)
	if err != nil {
		pm.Logger.Warn("Failed to send ping",
			zap.String("peerID", p.String()),
			zap.Error(err),
		)
		pm.MetricsCollector.UpdateResponsiveness(p, 0.0)
		pm.MetricsCollector.UpdateReliability(p, 0.0)
		return
	}

	// Await pong response
	buf := make([]byte, 1024)
	n, err := stream.Read(buf)
	if err != nil {
		pm.Logger.Warn("Failed to read pong",
			zap.String("peerID", p.String()),
			zap.Error(err),
		)
		pm.MetricsCollector.UpdateResponsiveness(p, 0.0)
		pm.MetricsCollector.UpdateReliability(p, 0.0)
		return
	}

	response := string(buf[:n])
	pm.Logger.Debug("Received response from peer", zap.String("peerID", p.String()), zap.String("response", response))
	if response != "pong" {
		pm.Logger.Warn("Invalid pong response",
			zap.String("peerID", p.String()),
			zap.String("response", response),
		)
		pm.MetricsCollector.UpdateResponsiveness(p, 0.0)
		pm.MetricsCollector.UpdateReliability(p, 0.0)
		return
	}

	// Calculate latency
	latency := time.Since(start).Seconds()

	// Update utility metrics based on ping success
	pm.MetricsCollector.UpdateResponsiveness(p, 1.0/latency) // Higher score for lower latency
	pm.MetricsCollector.UpdateReliability(p, 1.0)            // Full reliability on success

	pm.Logger.Info("Received pong from peer", zap.String("peerID", p.String()), zap.Float64("latency", latency))
}

// Stop halts the performance monitoring process gracefully.
func (pm *PerformanceMonitor) Stop() {
	pm.Logger.Info("Stopping PerformanceMonitor")
	pm.cancel()
	pm.wg.Wait()
	pm.Logger.Info("PerformanceMonitor stopped")
}
