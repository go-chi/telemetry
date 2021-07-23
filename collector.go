package telemetry

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
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
			if r.Method == "GET" {
				if isPathIgnored(r.URL.Path) {
					next.ServeHTTP(w, r)
					return
				}
				if strings.EqualFold(r.URL.Path, "/metrics") {
					// serve metrics page
					metricsHandler.Handler(next).ServeHTTP(w, r)
					return
				}
			}

			// measure request
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			defer sample(time.Now().UTC(), r, ww)

			next.ServeHTTP(ww, r)
		}
		return http.HandlerFunc(fn)
	}
}

var ignoredPaths = []string{"/ping", "/status"}

func isPathIgnored(path string) bool {
	for _, ignoredPath := range ignoredPaths {
		if strings.EqualFold(path, ignoredPath) {
			return true
		}
	}
	return false
}
