package telemetry

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Collector creates a handler that exposes a /metrics endpoint
func Collector(cfg Config) func(next http.Handler) http.Handler {
	if (!cfg.AllowAny && !cfg.AllowInternal) && (cfg.Username == "" || cfg.Password == "") {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	authHandler := middleware.BasicAuth(
		"metrics",
		map[string]string{cfg.Username: cfg.Password},
	)

	metricsHandler := chi.Chain(
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

				// Maybe allow internal traffic

				// TODO/NOTE: we've seen in some cases on GCP internal traffic is being blocked, therefore
				// our checks for ip parsing and checking against a private subnet has a minor bug

				if cfg.AllowInternal {
					ipAddress := net.ParseIP(getIPAddress(r))
					if isPrivateSubnet(ipAddress) {
						next.ServeHTTP(w, r)
						return
					}
				}

				// Maybw allow basic auth traffic
				if cfg.Username != "" && cfg.Password != "" {
					authHandler(next).ServeHTTP(w, r)
					return
				}

				// Maybe allow any
				if cfg.AllowAny {
					next.ServeHTTP(w, r)
					return
				}

				w.WriteHeader(http.StatusNotFound)
			})
		},
		func(http.Handler) http.Handler {
			return reporter.HTTPHandler()
		},
	)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" && strings.EqualFold(r.URL.Path, "/metrics") {
				// serve metrics page
				metricsHandler.Handler(next).ServeHTTP(w, r)
				return
			}

			// measure request
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			defer sample(time.Now().UTC(), r, ww)

			next.ServeHTTP(ww, r)
		})
	}
}
