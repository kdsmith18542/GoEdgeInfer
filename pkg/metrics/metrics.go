package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Per-model request counter
	InferenceRequestCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "goedgeinfer_inference_requests_total",
		Help: "The total number of inference requests (by model)",
	}, []string{"model_id"})

	// Per-model latency histogram
	InferenceDurationHistogram = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "goedgeinfer_inference_duration_seconds",
		Help:    "Duration of inference requests in seconds (by model)",
		Buckets: prometheus.DefBuckets,
	}, []string{"model_id"})

	// Per-model error counter
	InferenceErrorsCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "goedgeinfer_inference_errors_total",
		Help: "The total number of inference errors (by model)",
	}, []string{"model_id"})

	// Queue depth gauge
	QueueDepthGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "goedgeinfer_queue_depth",
		Help: "Current depth of the inference request queue",
	})

	// Per-model load counter (with status)
	ModelLoadCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "goedgeinfer_model_load_total",
		Help: "The total number of model load attempts (by model, status)",
	}, []string{"model_id", "version", "status"})

	// Per-model unload counter (with status)
	ModelUnloadCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "goedgeinfer_model_unload_total",
		Help: "The total number of model unload attempts (by model, status)",
	}, []string{"model_id", "version", "status"})

	// Per-model model load duration
	ModelLoadDurationHistogram = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "goedgeinfer_model_load_duration_seconds",
		Help:    "Duration of model load operations in seconds (by model)",
		Buckets: prometheus.DefBuckets,
	}, []string{"model_id", "version"})
)

// RecordInferenceDuration records the duration of an inference request in seconds for a given model
func RecordInferenceDuration(modelID string, seconds float64) {
	InferenceDurationHistogram.WithLabelValues(modelID).Observe(seconds)
}
