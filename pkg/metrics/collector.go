package metrics

import (
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

// Collector collects and updates utility metrics.
type Collector struct {
	metrics map[peer.ID]*Metrics
	mu      sync.RWMutex
	weights Metrics
}

// NewCollector initializes a new MetricsCollector with the provided weights.
func NewCollector(weights Metrics) *Collector {
	return &Collector{
		metrics: make(map[peer.ID]*Metrics),
		weights: weights,
	}
}

// UpdateResponsiveness updates the responsiveness metric for a peer.
func (mc *Collector) UpdateResponsiveness(p peer.ID, value float64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.ensurePeer(p)
	mc.metrics[p].Responsiveness = value
	mc.metrics[p].LastUpdated = time.Now()
}

// UpdateReliability updates the reliability metric for a peer.
func (mc *Collector) UpdateReliability(p peer.ID, value float64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.ensurePeer(p)
	mc.metrics[p].Reliability = value
	mc.metrics[p].LastUpdated = time.Now()
}

// UpdateBandwidthUsage updates the bandwidth usage metric for a peer.
func (mc *Collector) UpdateBandwidthUsage(p peer.ID, value float64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.ensurePeer(p)
	mc.metrics[p].BandwidthUsage = value
	mc.metrics[p].LastUpdated = time.Now()
}

// UpdateComputational updates the computational metric for a peer.
func (mc *Collector) UpdateComputational(p peer.ID, value float64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.ensurePeer(p)
	mc.metrics[p].Computational = value
	mc.metrics[p].LastUpdated = time.Now()
}

// UpdateStorage updates the storage metric for a peer.
func (mc *Collector) UpdateStorage(p peer.ID, value float64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.ensurePeer(p)
	mc.metrics[p].Storage = value
	mc.metrics[p].LastUpdated = time.Now()
}

// UpdateUptime updates the uptime metric for a peer.
func (mc *Collector) UpdateUptime(p peer.ID, value float64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.ensurePeer(p)
	mc.metrics[p].Uptime = value
	mc.metrics[p].LastUpdated = time.Now()
}

// ensurePeer initializes metrics for a peer if not already present.
func (mc *Collector) ensurePeer(p peer.ID) {
	if _, exists := mc.metrics[p]; !exists {
		mc.metrics[p] = &Metrics{}
	}
}

// CalculateUtilityScore computes the overall utility score for a peer.
func (mc *Collector) CalculateUtilityScore(p peer.ID) float64 {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	mc.ensurePeer(p)

	m := mc.metrics[p]
	w := mc.weights

	utilityScore := (m.BandwidthUsage * w.BandwidthUsage) +
		(m.Computational * w.Computational) +
		(m.Storage * w.Storage) +
		(m.Uptime * w.Uptime) +
		(m.Responsiveness * w.Responsiveness) +
		(m.Reliability * w.Reliability)

	return utilityScore
}

// GetMetrics retrieves the metrics for a given peer.
func (mc *Collector) GetMetrics(p peer.ID) *Metrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	if m, exists := mc.metrics[p]; exists {
		return m
	}
	return nil
}

// GetAllMetrics returns a copy of all collected metrics
func (mc *Collector) GetAllMetrics() map[peer.ID]*Metrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	// Create a copy to avoid race conditions
	metricsCopy := make(map[peer.ID]*Metrics)
	for p, m := range mc.metrics {
		// Deep copy the metrics
		metricsCopy[p] = &Metrics{
			BandwidthUsage: m.BandwidthUsage,
			Computational:  m.Computational,
			Storage:        m.Storage,
			Uptime:         m.Uptime,
			Responsiveness: m.Responsiveness,
			Reliability:    m.Reliability,
			LastUpdated:    m.LastUpdated,
		}
	}
	return metricsCopy
}
