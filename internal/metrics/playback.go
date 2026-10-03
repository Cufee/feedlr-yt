package metrics

import "github.com/prometheus/client_golang/prometheus"

var playbackEvents = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: "feedlr", Subsystem: "playback", Name: "events_total",
	Help: "Playback resolution, health, media and browser outcomes by reason.",
}, []string{"event", "reason"})

var playbackStartup = prometheus.NewHistogram(prometheus.HistogramOpts{
	Namespace: "feedlr", Subsystem: "playback", Name: "startup_seconds",
	Help:    "Browser native playback time to first frame.",
	Buckets: []float64{0.5, 1, 2, 4, 8, 10, 15, 30},
})

func init() { prometheus.MustRegister(playbackEvents, playbackStartup) }

// ObservePlayback accepts server-controlled labels only. Browser events must
// pass the route's finite allowlist before reaching this function.
func ObservePlayback(event, reason string) { playbackEvents.WithLabelValues(event, reason).Inc() }

func ObservePlaybackStartup(seconds float64) { playbackStartup.Observe(seconds) }
