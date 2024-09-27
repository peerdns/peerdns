// pkg/topology/metrics.go
package topology

import (
	"math"
	"sync"
	"time"
)

// MetricsCollector collects and manages metrics for peers in the topology.
type MetricsCollector struct {
	peerLatencies map[string]time.Duration
	peerBandwidth map[string]float64
	mu            sync.RWMutex
}

// NewMetricsCollector initializes a new MetricsCollector.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		peerLatencies: make(map[string]time.Duration),
		peerBandwidth: make(map[string]float64),
	}
}

// RecordLatency records the latency for a specific peer.
func (mc *MetricsCollector) RecordLatency(peerID string, latency time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.peerLatencies[peerID] = latency
}

// RecordBandwidth records the bandwidth for a specific peer.
func (mc *MetricsCollector) RecordBandwidth(peerID string, bandwidth float64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.peerBandwidth[peerID] = bandwidth
}

// GetLatency returns the latency for a specific peer.
func (mc *MetricsCollector) GetLatency(peerID string) (time.Duration, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	latency, exists := mc.peerLatencies[peerID]
	return latency, exists
}

// GetBandwidth returns the bandwidth for a specific peer.
func (mc *MetricsCollector) GetBandwidth(peerID string) (float64, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	bandwidth, exists := mc.peerBandwidth[peerID]
	return bandwidth, exists
}

// CalculateAverageLatency calculates the average latency across all peers.
func (mc *MetricsCollector) CalculateAverageLatency() time.Duration {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	var totalLatency time.Duration
	for _, latency := range mc.peerLatencies {
		totalLatency += latency
	}

	if len(mc.peerLatencies) == 0 {
		return 0
	}

	return totalLatency / time.Duration(len(mc.peerLatencies))
}

// CalculateTotalBandwidth calculates the total bandwidth across all peers.
func (mc *MetricsCollector) CalculateTotalBandwidth() float64 {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	var totalBandwidth float64
	for _, bandwidth := range mc.peerBandwidth {
		totalBandwidth += bandwidth
	}

	return totalBandwidth
}

// GetMinLatency returns the minimum latency among all peers.
func (mc *MetricsCollector) GetMinLatency() time.Duration {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	minLatency := time.Duration(math.MaxInt64)
	for _, latency := range mc.peerLatencies {
		if latency < minLatency {
			minLatency = latency
		}
	}

	if minLatency == time.Duration(math.MaxInt64) {
		return 0
	}
	return minLatency
}

// GetMaxBandwidth returns the maximum bandwidth among all peers.
func (mc *MetricsCollector) GetMaxBandwidth() float64 {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	maxBandwidth := 0.0
	for _, bandwidth := range mc.peerBandwidth {
		if bandwidth > maxBandwidth {
			maxBandwidth = bandwidth
		}
	}

	return maxBandwidth
}
