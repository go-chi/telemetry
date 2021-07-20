package telemetry

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

// Collector creates a handler that exposes a /metrics endpoint
func Collector(cfg Config) func(next http.Handler) http.Handler {
	if cfg.Username == "" || cfg.Password == "" {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	metricsHandler := chi.Chain(
		middleware.BasicAuth(
			"metrics",
			map[string]string{cfg.Username: cfg.Password},
		),
		func(http.Handler) http.Handler {
			return reporter.HTTPHandler()
		},
	)

	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			defer sample(time.Now().UTC(), r, ww)

			if r.Method == "GET" && isPathIgnored(r.URL.Path) {
				metricsHandler.Handler(next).ServeHTTP(ww, r)
				return
			}

			next.ServeHTTP(ww, r)
		}
		return http.HandlerFunc(fn)
	}
}

var ignoredPaths = []string{"/metrics", "/ping", "/status"}

func isPathIgnored(path string) bool {
	for _, ignoredPath := range ignoredPaths {
		if strings.EqualFold(path, ignoredPath) {
			return true
		}
	}
	return false
}
