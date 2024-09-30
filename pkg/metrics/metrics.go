package metrics

import "time"

// Metrics represents the utility metrics for a validator.
type Metrics struct {
	BandwidthUsage float64 // bytes per second
	Computational  float64
	Storage        float64
	Uptime         float64 // hours
	Responsiveness float64
	Reliability    float64
	LastUpdated    time.Time
}
