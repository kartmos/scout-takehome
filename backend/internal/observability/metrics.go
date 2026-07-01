package observability

import "github.com/prometheus/client_golang/prometheus"

// Metrics holds all application Prometheus metrics.
// Construct with NewMetrics using a fresh registry to prevent global-registration panics.
type Metrics struct {
	// HTTP request metrics; labels: method, route, status_class.
	HTTPRequestsTotal   *prometheus.CounterVec
	HTTPDurationSeconds *prometheus.HistogramVec

	// Thumbnail cache metrics (no high-cardinality labels).
	ThumbCacheHitsTotal   prometheus.Counter
	ThumbCacheMissesTotal prometheus.Counter

	// Thumbnail generation metrics (no high-cardinality labels).
	ThumbGenTotal       prometheus.Counter
	ThumbGenErrorsTotal prometheus.Counter
	ThumbGenDuration    prometheus.Histogram
}

// CacheStatsFunc is called at scrape time to read current cache state.
// It must not block or trigger generation.
type CacheStatsFunc func() (bytes int64, entries int, evictions int64)

// evictionsCollector is a scrape-time counter backed by a CacheStatsFunc.
// Using a custom Collector produces a proper counter-type metric without
// requiring callers to manually increment a Counter.
type evictionsCollector struct {
	desc      *prometheus.Desc
	statsFunc CacheStatsFunc
}

func (c *evictionsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

func (c *evictionsCollector) Collect(ch chan<- prometheus.Metric) {
	_, _, evictions := c.statsFunc()
	ch <- prometheus.MustNewConstMetric(c.desc, prometheus.CounterValue, float64(evictions))
}

// NewMetrics registers all application metrics against reg.
// statsFunc, if non-nil, is called at scrape time to populate cache byte/entry gauges
// and the eviction counter. Returns an error if any descriptor conflicts with an
// existing registration in reg.
func NewMetrics(reg prometheus.Registerer, statsFunc CacheStatsFunc) (*Metrics, error) {
	m := &Metrics{
		HTTPRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "scout_http_requests_total",
			Help: "Total HTTP requests by method, route, and status class.",
		}, []string{"method", "route", "status_class"}),

		HTTPDurationSeconds: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "scout_http_request_duration_seconds",
			Help:    "HTTP request latency by method, route, and status class.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "route", "status_class"}),

		ThumbCacheHitsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "scout_thumbnail_cache_hits_total",
			Help: "Thumbnail cache hits (served from disk without generation).",
		}),

		ThumbCacheMissesTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "scout_thumbnail_cache_misses_total",
			Help: "Thumbnail cache misses: every request whose initial lookup finds no committed entry, including coalesced followers. Exceeds generation count during concurrent cold bursts.",
		}),

		ThumbGenTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "scout_thumbnail_generations_total",
			Help: "Total thumbnail generation attempts (successful and failed).",
		}),

		ThumbGenErrorsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "scout_thumbnail_generation_errors_total",
			Help: "Failed thumbnail generation attempts.",
		}),

		ThumbGenDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "scout_thumbnail_generation_duration_seconds",
			Help:    "Wall-clock time spent on thumbnail generation (decode+resize+encode).",
			Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10},
		}),
	}

	collectors := []prometheus.Collector{
		m.HTTPRequestsTotal,
		m.HTTPDurationSeconds,
		m.ThumbCacheHitsTotal,
		m.ThumbCacheMissesTotal,
		m.ThumbGenTotal,
		m.ThumbGenErrorsTotal,
		m.ThumbGenDuration,
	}

	if statsFunc != nil {
		collectors = append(collectors,
			prometheus.NewGaugeFunc(prometheus.GaugeOpts{
				Name: "scout_thumbnail_cache_bytes",
				Help: "Current thumbnail cache disk usage in bytes.",
			}, func() float64 {
				b, _, _ := statsFunc()
				return float64(b)
			}),
			prometheus.NewGaugeFunc(prometheus.GaugeOpts{
				Name: "scout_thumbnail_cache_entries",
				Help: "Current number of entries in the thumbnail cache.",
			}, func() float64 {
				_, e, _ := statsFunc()
				return float64(e)
			}),
			&evictionsCollector{
				desc: prometheus.NewDesc(
					"scout_thumbnail_cache_evictions_total",
					"Thumbnail cache entries evicted to stay within the disk budget.",
					nil, nil,
				),
				statsFunc: statsFunc,
			},
		)
	}

	for _, c := range collectors {
		if err := reg.Register(c); err != nil {
			return nil, err
		}
	}

	return m, nil
}
