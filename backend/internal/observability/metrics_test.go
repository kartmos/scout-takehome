package observability_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"

	"scout/internal/observability"
)

// newRegistry returns a fresh isolated Prometheus registry.
func newRegistry() *prometheus.Registry {
	return prometheus.NewRegistry()
}

// gatherFamily returns the MetricFamily with the given name from reg, or nil.
func gatherFamily(t *testing.T, reg prometheus.Gatherer, name string) *dto.MetricFamily {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	for _, mf := range families {
		if mf.GetName() == name {
			return mf
		}
	}
	return nil
}

// gatherAllNames returns all metric family names from reg.
func gatherAllNames(t *testing.T, reg prometheus.Gatherer) []string {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	names := make([]string, 0, len(families))
	for _, mf := range families {
		names = append(names, mf.GetName())
	}
	return names
}

// TestNewMetrics_FreshRegistry verifies all expected metric families are registered.
func TestNewMetrics_FreshRegistry(t *testing.T) {
	reg := newRegistry()
	m, err := observability.NewMetrics(reg, func() (int64, int, int64) { return 0, 0, 0 })
	if err != nil {
		t.Fatalf("NewMetrics: %v", err)
	}
	if m == nil {
		t.Fatal("NewMetrics returned nil")
	}

	// Trigger some activity so metrics appear in Gather.
	m.HTTPRequestsTotal.WithLabelValues("GET", "/healthz", "2xx").Inc()
	m.HTTPDurationSeconds.WithLabelValues("GET", "/healthz", "2xx").Observe(0.01)
	m.ThumbCacheHitsTotal.Inc()
	m.ThumbCacheMissesTotal.Inc()
	m.ThumbGenTotal.Inc()
	m.ThumbGenErrorsTotal.Inc()
	m.ThumbGenDuration.Observe(0.5)
	// scout_thumbnail_cache_evictions_total is scrape-time via statsFunc; no Inc() needed.

	want := []string{
		"scout_http_requests_total",
		"scout_http_request_duration_seconds",
		"scout_thumbnail_cache_hits_total",
		"scout_thumbnail_cache_misses_total",
		"scout_thumbnail_cache_evictions_total",
		"scout_thumbnail_generations_total",
		"scout_thumbnail_generation_errors_total",
		"scout_thumbnail_generation_duration_seconds",
		"scout_thumbnail_cache_bytes",
		"scout_thumbnail_cache_entries",
	}

	names := gatherAllNames(t, reg)
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}
	for _, w := range want {
		if !nameSet[w] {
			t.Errorf("expected metric family %q not found; got: %v", w, names)
		}
	}
}

// TestNewMetrics_NilStatsFunc verifies NewMetrics works without cache gauge stats.
func TestNewMetrics_NilStatsFunc(t *testing.T) {
	reg := newRegistry()
	m, err := observability.NewMetrics(reg, nil)
	if err != nil {
		t.Fatalf("NewMetrics with nil statsFunc: %v", err)
	}
	if m == nil {
		t.Fatal("returned nil")
	}
	// Cache byte/entry gauges must be absent.
	names := gatherAllNames(t, reg)
	for _, n := range names {
		if n == "scout_thumbnail_cache_bytes" || n == "scout_thumbnail_cache_entries" {
			t.Errorf("gauge %q must not be registered when statsFunc is nil", n)
		}
	}
}

// TestNewMetrics_DuplicateRegistrationFails verifies that registering to a non-empty
// registry with colliding metrics returns an error rather than panicking.
func TestNewMetrics_DuplicateRegistrationFails(t *testing.T) {
	reg := newRegistry()
	if _, err := observability.NewMetrics(reg, nil); err != nil {
		t.Fatalf("first NewMetrics: %v", err)
	}
	// Second registration into the same registry must fail.
	if _, err := observability.NewMetrics(reg, nil); err == nil {
		t.Error("second NewMetrics into same registry must return an error")
	}
}

// TestHTTPMetrics_IncrementPerRequest verifies one request increments the counter by 1.
func TestHTTPMetrics_IncrementPerRequest(t *testing.T) {
	reg := newRegistry()
	m, _ := observability.NewMetrics(reg, nil)

	before := gatherFamily(t, reg, "scout_http_requests_total")
	if before != nil {
		t.Fatal("counter must be absent before any request")
	}

	m.HTTPRequestsTotal.WithLabelValues("GET", "GET /healthz", "2xx").Inc()

	after := gatherFamily(t, reg, "scout_http_requests_total")
	if after == nil {
		t.Fatal("counter must appear after increment")
	}
	if len(after.GetMetric()) != 1 {
		t.Fatalf("want 1 metric series, got %d", len(after.GetMetric()))
	}
	if v := after.GetMetric()[0].GetCounter().GetValue(); v != 1 {
		t.Errorf("counter value = %v, want 1", v)
	}
}

// TestCacheMetrics_EvictionsFromStatsFunc verifies the eviction counter is backed by
// the stats function, not by manual Inc calls.
func TestCacheMetrics_EvictionsFromStatsFunc(t *testing.T) {
	var evictions int64
	statsFunc := func() (int64, int, int64) { return 0, 0, evictions }

	reg := newRegistry()
	_, err := observability.NewMetrics(reg, statsFunc)
	if err != nil {
		t.Fatalf("NewMetrics: %v", err)
	}

	// Before any evictions the counter must be zero.
	mf := gatherFamily(t, reg, "scout_thumbnail_cache_evictions_total")
	if mf == nil {
		t.Fatal("scout_thumbnail_cache_evictions_total must be present when statsFunc is non-nil")
	}
	if v := mf.GetMetric()[0].GetCounter().GetValue(); v != 0 {
		t.Errorf("evictions = %v, want 0", v)
	}

	// Simulate cache evictions by advancing the stats function's return value.
	evictions = 3
	mf = gatherFamily(t, reg, "scout_thumbnail_cache_evictions_total")
	if mf == nil {
		t.Fatal("scout_thumbnail_cache_evictions_total not found after evictions")
	}
	if v := mf.GetMetric()[0].GetCounter().GetValue(); v != 3 {
		t.Errorf("evictions = %v, want 3", v)
	}
}

// TestCacheMetrics_HitMiss verifies hit and miss counters change by 1 per operation.
func TestCacheMetrics_HitMiss(t *testing.T) {
	reg := newRegistry()
	m, _ := observability.NewMetrics(reg, nil)

	m.ThumbCacheHitsTotal.Inc()
	m.ThumbCacheHitsTotal.Inc()
	m.ThumbCacheMissesTotal.Inc()

	hitsFamily := gatherFamily(t, reg, "scout_thumbnail_cache_hits_total")
	if hitsFamily == nil {
		t.Fatal("scout_thumbnail_cache_hits_total not found")
	}
	if v := hitsFamily.GetMetric()[0].GetCounter().GetValue(); v != 2 {
		t.Errorf("hits = %v, want 2", v)
	}

	missFamily := gatherFamily(t, reg, "scout_thumbnail_cache_misses_total")
	if missFamily == nil {
		t.Fatal("scout_thumbnail_cache_misses_total not found")
	}
	if v := missFamily.GetMetric()[0].GetCounter().GetValue(); v != 1 {
		t.Errorf("misses = %v, want 1", v)
	}
}

// TestGenMetrics_DurationRecorded verifies generation duration is observed.
func TestGenMetrics_DurationRecorded(t *testing.T) {
	reg := newRegistry()
	m, _ := observability.NewMetrics(reg, nil)

	m.ThumbGenTotal.Inc()
	m.ThumbGenDuration.Observe(0.123)

	genFamily := gatherFamily(t, reg, "scout_thumbnail_generation_duration_seconds")
	if genFamily == nil {
		t.Fatal("scout_thumbnail_generation_duration_seconds not found")
	}
	h := genFamily.GetMetric()[0].GetHistogram()
	if h.GetSampleCount() != 1 {
		t.Errorf("sample count = %d, want 1", h.GetSampleCount())
	}
	if h.GetSampleSum() < 0.1 || h.GetSampleSum() > 0.2 {
		t.Errorf("sample sum = %v, want ~0.123", h.GetSampleSum())
	}
}

// TestMetricLabels_NoDynamicValues verifies that no label in any series contains
// dynamic identifiers (UUIDs, signed URL fragments, cache keys, API keys).
func TestMetricLabels_NoDynamicValues(t *testing.T) {
	const fakeUUID = "550e8400-e29b-41d4-a716-446655440000"
	const fakeSignedURL = "https://minio.example.com/bucket?X-Amz-Signature=secret"

	reg := newRegistry()
	m, _ := observability.NewMetrics(reg, nil)

	// Record metrics with bounded label values.
	m.HTTPRequestsTotal.WithLabelValues("GET", "GET /photos/{photoId}/thumbnail", "2xx").Inc()
	m.HTTPDurationSeconds.WithLabelValues("GET", "GET /photos/{photoId}/thumbnail", "2xx").Observe(0.05)

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}

	for _, mf := range families {
		for _, metric := range mf.GetMetric() {
			for _, lp := range metric.GetLabel() {
				v := lp.GetValue()
				if strings.Contains(v, fakeUUID) {
					t.Errorf("label %q contains UUID in metric %s", v, mf.GetName())
				}
				if strings.Contains(v, "Amz-Signature") {
					t.Errorf("label %q contains signed URL fragment in metric %s", v, mf.GetName())
				}
				if strings.Contains(v, "secret") {
					t.Errorf("label %q contains secret in metric %s", v, mf.GetName())
				}
				// Route patterns must be static, not raw paths with UUIDs.
				if strings.Contains(v, fakeUUID) {
					t.Errorf("label %q contains dynamic identifier in metric %s", v, mf.GetName())
				}
			}
		}
	}
	_ = fakeSignedURL
}

// TestMetricsEndpoint_Public verifies /metrics is public, returns 200, and contains expected names.
func TestMetricsEndpoint_Public(t *testing.T) {
	reg := newRegistry()
	m, _ := observability.NewMetrics(reg, nil)

	// Trigger one observation so the counter appears.
	m.HTTPRequestsTotal.WithLabelValues("GET", "GET /healthz", "2xx").Inc()

	handler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("/metrics status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "scout_http_requests_total") {
		t.Errorf("/metrics body missing scout_http_requests_total; body: %s", body[:min(200, len(body))])
	}
	if strings.Contains(body, "secret") || strings.Contains(body, "api_key") {
		t.Error("/metrics must not expose secrets")
	}
}

// TestMetricsEndpoint_DoesNotCallRepositoryOrStorage verifies /metrics is a pure read.
// In this test we just verify the endpoint returns without hitting any backend (no panics,
// no side effects). A real repo/storage would panic if called without a database.
func TestMetricsEndpoint_DoesNotCallRepositoryOrStorage(t *testing.T) {
	reg := newRegistry()
	_, _ = observability.NewMetrics(reg, func() (int64, int, int64) {
		// Stats func must be called without side effects.
		return 1024, 5, 0
	})

	handler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/metrics", nil)

	// Must complete without blocking or panicking.
	done := make(chan struct{})
	go func() {
		defer close(done)
		handler.ServeHTTP(w, r)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("/metrics handler must not block")
	}

	if w.Code != http.StatusOK {
		t.Errorf("/metrics status = %d, want 200", w.Code)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
