// pkg/utility/utility_calculator.go
package utility

import (
	"sync"
	"time"
)

// Metrics represents the utility metrics for a validator.
type Metrics struct {
	Bandwidth      float64
	Computational  float64
	Storage        float64
	Uptime         float64
	Responsiveness float64
	Reliability    float64
}

// UtilityCalculator calculates and updates utility metrics.
type UtilityCalculator struct {
	metrics Metrics
	mu      sync.RWMutex
}

// NewUtilityCalculator initializes a new UtilityCalculator with the provided initial metrics.
func NewUtilityCalculator(initial Metrics) *UtilityCalculator {
	return &UtilityCalculator{
		metrics: initial,
	}
}

// UpdateBandwidth updates the bandwidth metric.
func (uc *UtilityCalculator) UpdateBandwidth(value float64) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.metrics.Bandwidth = value
}

// UpdateComputational updates the computational metric.
func (uc *UtilityCalculator) UpdateComputational(value float64) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.metrics.Computational = value
}

// UpdateStorage updates the storage metric.
func (uc *UtilityCalculator) UpdateStorage(value float64) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.metrics.Storage = value
}

// UpdateUptime updates the uptime metric.
func (uc *UtilityCalculator) UpdateUptime(duration time.Duration) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.metrics.Uptime = duration.Hours()
}

// UpdateResponsiveness updates the responsiveness metric.
func (uc *UtilityCalculator) UpdateResponsiveness(value float64) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.metrics.Responsiveness = value
}

// UpdateReliability updates the reliability metric.
func (uc *UtilityCalculator) UpdateReliability(value float64) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.metrics.Reliability = value
}

// GetMetrics retrieves the current utility metrics.
func (uc *UtilityCalculator) GetMetrics() Metrics {
	uc.mu.RLock()
	defer uc.mu.RUnlock()
	return uc.metrics
}

// CalculateOverallUtility calculates the overall utility based on weighted metrics.
func (uc *UtilityCalculator) CalculateOverallUtility(weights Metrics) float64 {
	uc.mu.RLock()
	defer uc.mu.RUnlock()

	utility := (uc.metrics.Bandwidth * weights.Bandwidth) +
		(uc.metrics.Computational * weights.Computational) +
		(uc.metrics.Storage * weights.Storage) +
		(uc.metrics.Uptime * weights.Uptime) +
		(uc.metrics.Responsiveness * weights.Responsiveness) +
		(uc.metrics.Reliability * weights.Reliability)

	return utility
}
