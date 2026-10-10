package metrics

import "github.com/prometheus/client_golang/prometheus"

var podcastProcessingStageDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: "feedlr", Subsystem: "podcast_processing", Name: "stage_duration_seconds", Help: "Podcast preparation and scanning stage duration.",
	Buckets: []float64{1, 5, 10, 20, 30, 60, 120, 300, 600, 1800},
}, []string{"stage", "outcome"})
var podcastProviderCost = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "feedlr", Subsystem: "podcast_processing", Name: "provider_cost_usd_total", Help: "Provider reported cost of successful podcast requests in USD; reconcile retries with provider billing.",
}, []string{"operation"})

func init() { prometheus.MustRegister(podcastProcessingStageDuration, podcastProviderCost) }
func ObservePodcastProcessingStage(stage, outcome string, seconds float64) {
	podcastProcessingStageDuration.WithLabelValues(stage, outcome).Observe(seconds)
}
func ObservePodcastProviderCost(operation string, cost float64) {
	if cost > 0 {
		podcastProviderCost.WithLabelValues(operation).Add(cost)
	}
}
