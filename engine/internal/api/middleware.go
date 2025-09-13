package engine

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// метрики

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "engine_http_requests_total",
			Help: "Кол-во HTTP запросов",
		},
		[]string{"path", "code"},
	)

	httpRequestsError = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "engine_http_errors_total",
			Help: "Кол-во ошибочных HTTP запросов",
		},
		[]string{"path", "code"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "engine_http_request_duration_seconds",
			Help:    "Продолжительность HTTP запросов",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"path", "code"},
	)
)

// логируем вызовы
type logResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *logResponseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func MiddlewareLog() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			reqtime := time.Now()
			logrw := &logResponseWriter{w, 200}
			next.ServeHTTP(logrw, r)

			labels := prometheus.Labels{
				"path": r.URL.Path,
				"code": strconv.Itoa(logrw.status),
			}
			httpRequestsTotal.With(labels).Inc()
			httpRequestDuration.With(labels).Observe(time.Since(reqtime).Seconds())

			if logrw.status != http.StatusOK {
				httpRequestsError.With(labels).Inc()
			}
		})
	}
}
