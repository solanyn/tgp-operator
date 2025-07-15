// Package metrics provides Prometheus metrics for the TGP operator
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	subsystem = "tgp_operator"
)

var (
	// GPU request metrics
	gpuRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: subsystem,
			Name:      "gpu_requests_total",
			Help:      "Total number of GPU requests processed",
		},
		[]string{"provider", "gpu_type", "region", "phase"},
	)

	gpuRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: subsystem,
			Name:      "gpu_request_duration_seconds",
			Help:      "Time spent processing GPU requests",
			Buckets:   prometheus.ExponentialBuckets(1, 2, 10), // 1s to ~17min
		},
		[]string{"provider", "gpu_type", "phase"},
	)

	// Instance lifecycle metrics
	instanceLaunchDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: subsystem,
			Name:      "instance_launch_duration_seconds",
			Help:      "Time taken to launch GPU instances",
			Buckets:   prometheus.ExponentialBuckets(10, 2, 8), // 10s to ~43min
		},
		[]string{"provider", "gpu_type", "region"},
	)

	instancesActive = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: subsystem,
			Name:      "instances_active",
			Help:      "Number of active GPU instances",
		},
		[]string{"provider", "gpu_type", "region"},
	)

	// Cost metrics
	instanceHourlyCost = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: subsystem,
			Name:      "instance_hourly_cost_usd",
			Help:      "Current hourly cost of GPU instances in USD",
		},
		[]string{"provider", "gpu_type", "region"},
	)

	// Provider metrics
	providerRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: subsystem,
			Name:      "provider_requests_total",
			Help:      "Total number of requests to cloud providers",
		},
		[]string{"provider", "operation", "status"},
	)

	providerRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: subsystem,
			Name:      "provider_request_duration_seconds",
			Help:      "Duration of requests to cloud providers",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"provider", "operation"},
	)

	// Health check metrics
	healthChecksTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: subsystem,
			Name:      "health_checks_total",
			Help:      "Total number of instance health checks",
		},
		[]string{"provider", "status"},
	)

	// Idle timeout metrics
	idleTimeoutsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: subsystem,
			Name:      "idle_timeouts_total",
			Help:      "Total number of instances terminated due to idle timeout",
		},
		[]string{"provider", "gpu_type"},
	)
)

// RegisterMetrics registers all metrics with the controller-runtime metrics registry
func RegisterMetrics() {
	metrics.Registry.MustRegister(
		gpuRequestsTotal,
		gpuRequestDuration,
		instanceLaunchDuration,
		instancesActive,
		instanceHourlyCost,
		providerRequests,
		providerRequestDuration,
		healthChecksTotal,
		idleTimeoutsTotal,
	)
}

// Metrics provides methods to record various operator metrics
type Metrics struct{}

// NewMetrics creates a new Metrics instance
func NewMetrics() *Metrics {
	return &Metrics{}
}

// RecordGPURequest records a GPU request with its details
func (m *Metrics) RecordGPURequest(provider, gpuType, region, phase string) {
	gpuRequestsTotal.WithLabelValues(provider, gpuType, region, phase).Inc()
}

// RecordGPURequestDuration records the duration of processing a GPU request
func (m *Metrics) RecordGPURequestDuration(provider, gpuType, phase string, duration float64) {
	gpuRequestDuration.WithLabelValues(provider, gpuType, phase).Observe(duration)
}

// RecordInstanceLaunch records an instance launch with duration
func (m *Metrics) RecordInstanceLaunch(provider, gpuType, region string, duration float64) {
	instanceLaunchDuration.WithLabelValues(provider, gpuType, region).Observe(duration)
}

// SetInstanceActive sets the number of active instances
func (m *Metrics) SetInstanceActive(provider, gpuType, region string, count float64) {
	instancesActive.WithLabelValues(provider, gpuType, region).Set(count)
}

// SetInstanceCost sets the hourly cost for an instance
func (m *Metrics) SetInstanceCost(provider, gpuType, region string, cost float64) {
	instanceHourlyCost.WithLabelValues(provider, gpuType, region).Set(cost)
}

// RecordProviderRequest records a request to a cloud provider
func (m *Metrics) RecordProviderRequest(provider, operation, status string) {
	providerRequests.WithLabelValues(provider, operation, status).Inc()
}

// RecordProviderRequestDuration records the duration of a provider request
func (m *Metrics) RecordProviderRequestDuration(provider, operation string, duration float64) {
	providerRequestDuration.WithLabelValues(provider, operation).Observe(duration)
}

// RecordHealthCheck records a health check result
func (m *Metrics) RecordHealthCheck(provider, status string) {
	healthChecksTotal.WithLabelValues(provider, status).Inc()
}

// RecordIdleTimeout records an instance terminated due to idle timeout
func (m *Metrics) RecordIdleTimeout(provider, gpuType string) {
	idleTimeoutsTotal.WithLabelValues(provider, gpuType).Inc()
}
