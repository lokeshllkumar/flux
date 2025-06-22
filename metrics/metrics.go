package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

)

// counts the total calls to teh service registry
var RegistryCallsTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "flux_registry_calls_total",
		Help: "Total number of calls made to the service registry",
	},
	[]string{"operation", "protocol", "status"},
)

// records the duration of calls to the service registry
var RegistryCallDurationSeconds = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name: "flux_registry_call_duration_seconds",
		Help: "Duration of calls made to the service registry in seconds",
		Buckets: prometheus.DefBuckets,
	},
	[]string{"operation", "protocol", "status"},
)

// indicates the current state of the registrar
var RegistrarStateGauge = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "flux_registrar_state",
		Help: "Current state of the registrar (1 = running/registered, 0 = stopped/failed)",
	},
	[]string{"instance_id", "service_name"},
)

// registers all metrics with the default Prometheus registry
// expected to be called at application startup
func InitMetrics() {
	prometheus.MustRegister(RegistryCallsTotal)
	prometheus.MustRegister(RegistryCallDurationSeconds)
	prometheus.MustRegister(RegistrarStateGauge)
}

// return a HTTP handler that servers Prometheus metrics
// expected to be mounted by the main app's HTTP server
func Handler() http.Handler {
	return promhttp.Handler()
}
