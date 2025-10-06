package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/rsav/k8s-learning/internal/api/metrics"
)

// MetricsMiddleware records HTTP request metrics.
func MetricsMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status and size
			rw := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
				written:        0,
			}

			// Record request size
			if r.ContentLength > 0 {
				metrics.HTTPRequestSize.WithLabelValues(r.Method, r.URL.Path).Observe(float64(r.ContentLength))
			}

			// Process request
			next.ServeHTTP(rw, r)

			// Record metrics
			duration := time.Since(start).Seconds()
			status := strconv.Itoa(rw.statusCode)

			metrics.HTTPRequestsTotal.WithLabelValues(r.Method, r.URL.Path, status).Inc()
			metrics.HTTPRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
			metrics.HTTPResponseSize.WithLabelValues(r.Method, r.URL.Path).Observe(float64(rw.written))
		})
	}
}
