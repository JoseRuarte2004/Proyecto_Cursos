package metrics

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	registerOnce sync.Once

	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests handled by the service.",
		},
		[]string{"service", "method", "path", "status"},
	)
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "method", "path"},
	)
	dependencyErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dependency_errors_total",
			Help: "Total dependency communication errors.",
		},
		[]string{"service", "dependency"},
	)
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Flush() {
	flusher, ok := w.ResponseWriter.(http.Flusher)
	if !ok {
		return
	}
	flusher.Flush()
}

func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not support hijacking")
	}
	return hijacker.Hijack()
}

func (w *statusWriter) Push(target string, opts *http.PushOptions) error {
	pusher, ok := w.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, opts)
}

func (w *statusWriter) ReadFrom(r io.Reader) (int64, error) {
	readerFrom, ok := w.ResponseWriter.(io.ReaderFrom)
	if ok {
		return readerFrom.ReadFrom(r)
	}

	return io.Copy(w.ResponseWriter, r)
}

func register() {
	registerOnce.Do(func() {
		prometheus.MustRegister(httpRequestsTotal, httpRequestDuration, dependencyErrorsTotal)
	})
}

func Middleware(service string) func(http.Handler) http.Handler {
	register()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			writer := &statusWriter{
				ResponseWriter: w,
				status:         http.StatusOK,
			}

			next.ServeHTTP(writer, r)

			path := routePattern(r)
			httpRequestsTotal.WithLabelValues(service, r.Method, path, strconv.Itoa(writer.status)).Inc()
			httpRequestDuration.WithLabelValues(service, r.Method, path).Observe(time.Since(start).Seconds())
		})
	}
}

func Handler() http.Handler {
	register()
	return promhttp.Handler()
}

func RecordDependencyError(service, dependency string) {
	register()
	dependencyErrorsTotal.WithLabelValues(service, dependency).Inc()
}

func routePattern(r *http.Request) string {
	routeCtx := chi.RouteContext(r.Context())
	if routeCtx != nil {
		if pattern := routeCtx.RoutePattern(); pattern != "" {
			return pattern
		}
	}

	if r.URL.Path == "" {
		return "/"
	}

	return r.URL.Path
}
