package metrics

import (
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "feedlr",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests served by route.",
		},
		[]string{"method", "route", "status_class"},
	)

	usersTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "feedlr",
			Subsystem: "users",
			Name:      "events_total",
			Help:      "Total number of user lifecycle/auth events.",
		},
		[]string{"event", "outcome"},
	)

	userActionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "feedlr",
			Subsystem: "user_actions",
			Name:      "total",
			Help:      "Total number of user-triggered actions.",
		},
		[]string{"action", "outcome"},
	)

	youtubeAPICallsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "feedlr",
			Subsystem: "youtube_api",
			Name:      "calls_total",
			Help:      "Total number of YouTube API calls.",
		},
		[]string{"client", "operation", "outcome"},
	)

	youtubeOAuthCallsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "feedlr",
			Subsystem: "youtube_oauth",
			Name:      "calls_total",
			Help:      "Total number of YouTube OAuth client calls.",
		},
		[]string{"operation", "outcome"},
	)

	youtubeTVCallsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "feedlr",
			Subsystem: "youtube_tv",
			Name:      "calls_total",
			Help:      "Total number of YouTube TV lounge client calls.",
		},
		[]string{"operation", "outcome"},
	)

	videoRefreshTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "feedlr",
			Subsystem: "video_refresh",
			Name:      "operations_total",
			Help:      "Total number of video refresh/cache operations.",
		},
		[]string{"operation", "outcome"},
	)

	videoRefreshItemsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "feedlr",
			Subsystem: "video_refresh",
			Name:      "items_total",
			Help:      "Total number of videos processed by refresh/cache operations.",
		},
		[]string{"operation"},
	)

	backgroundTasksTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "feedlr",
			Subsystem: "background_tasks",
			Name:      "total",
			Help:      "Total number of background task executions.",
		},
		[]string{"task", "outcome"},
	)

	tvSyncEventsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "feedlr",
			Subsystem: "youtube_tv_sync",
			Name:      "events_total",
			Help:      "Total number of YouTube TV sync events.",
		},
		[]string{"event", "outcome"},
	)
)

func init() {
	prometheus.MustRegister(
		httpRequestsTotal,
		usersTotal,
		userActionsTotal,
		youtubeAPICallsTotal,
		youtubeOAuthCallsTotal,
		youtubeTVCallsTotal,
		videoRefreshTotal,
		videoRefreshItemsTotal,
		backgroundTasksTotal,
		tvSyncEventsTotal,
	)
}

func IncHTTPRequest(method, route string, statusCode int) {
	httpRequestsTotal.WithLabelValues(
		normalizeLabel(strings.ToUpper(method)),
		normalizeRoute(route),
		statusClass(statusCode),
	).Inc()
}

func IncUserEvent(event, outcome string) {
	usersTotal.WithLabelValues(normalizeLabel(event), normalizeLabel(outcome)).Inc()
}

func IncUserAction(action, outcome string) {
	userActionsTotal.WithLabelValues(normalizeLabel(action), normalizeLabel(outcome)).Inc()
}

func ObserveYouTubeAPICall(client, operation string, err error) {
	youtubeAPICallsTotal.WithLabelValues(
		normalizeLabel(client),
		normalizeLabel(operation),
		outcomeFromErr(err),
	).Inc()
}

func ObserveYouTubeOAuthCall(operation string, err error) {
	youtubeOAuthCallsTotal.WithLabelValues(
		normalizeLabel(operation),
		outcomeFromErr(err),
	).Inc()
}

func ObserveYouTubeTVCall(operation string, err error) {
	youtubeTVCallsTotal.WithLabelValues(
		normalizeLabel(operation),
		outcomeFromErr(err),
	).Inc()
}

func ObserveVideoRefresh(operation string, err error) {
	videoRefreshTotal.WithLabelValues(
		normalizeLabel(operation),
		outcomeFromErr(err),
	).Inc()
}

func AddVideoRefreshItems(operation string, count int) {
	if count <= 0 {
		return
	}
	videoRefreshItemsTotal.WithLabelValues(normalizeLabel(operation)).Add(float64(count))
}

func ObserveBackgroundTask(task string, err error) {
	backgroundTasksTotal.WithLabelValues(
		normalizeLabel(task),
		outcomeFromErr(err),
	).Inc()
}

func ObserveTVSyncEvent(event string, err error) {
	tvSyncEventsTotal.WithLabelValues(
		normalizeLabel(event),
		outcomeFromErr(err),
	).Inc()
}

func statusClass(statusCode int) string {
	if statusCode <= 0 {
		return "unknown"
	}
	return strconv.Itoa(statusCode/100) + "xx"
}

func outcomeFromErr(err error) string {
	if err != nil {
		return "error"
	}
	return "success"
}

func normalizeRoute(route string) string {
	route = strings.TrimSpace(route)
	if route == "" || route == "/*" {
		return "unknown"
	}
	return route
}

func normalizeLabel(label string) string {
	label = strings.TrimSpace(strings.ToLower(label))
	if label == "" {
		return "unknown"
	}
	return label
}
