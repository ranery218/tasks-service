package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type HTTPMetrics struct {
	requestsTotal   *prometheus.CounterVec
	errorsTotal     *prometheus.CounterVec
	durationSeconds *prometheus.HistogramVec
}

func NewHTTPMetrics(registry prometheus.Registerer) *HTTPMetrics {
	m := &HTTPMetrics{
		requestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		}, []string{"method", "path", "status"}),
		errorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_errors_total",
			Help: "Total number of HTTP error responses.",
		}, []string{"method", "path", "status"}),
		durationSeconds: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path", "status"}),
	}

	registry.MustRegister(m.requestsTotal, m.errorsTotal, m.durationSeconds)
	return m
}

func (m *HTTPMetrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		path := routePath(r)
		method := r.Method

		rw := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)

		status := strconv.Itoa(rw.statusCode)
		m.requestsTotal.WithLabelValues(method, path, status).Inc()
		if rw.statusCode >= 400 {
			m.errorsTotal.WithLabelValues(method, path, status).Inc()
		}
		m.durationSeconds.WithLabelValues(method, path, status).Observe(time.Since(start).Seconds())
	})
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (rw *statusRecorder) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

func routePath(r *http.Request) string {
	if pattern := r.Pattern; pattern != "" {
		return pattern
	}
	return r.URL.Path
}
