package telemetry

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Gives ability to exposes metrics on a separate http server.
// Example usage to expose `/metrics` endpoint on port `9093` using go std lib http server:
//
//	metricsServer := http.NewServeMux()
//	metricsServer.Handle("/metrics", telemetry.MetricsHandler())
//	err := http.ListenAndServe(":9093", metricsServer)
//
// If you want to also collect http request details as metrics,
// please remember to configure it by setting Config.CollectHttpRequestMetrics to true
func MetricsHandler() http.Handler {
	return reporter.HTTPHandler()
}

// Collector creates a handler that exposes a /metrics endpoint. Passing
// an array of strings to pathPrefixFilters will help reduce the noise on the service
// from random Internet traffic; that is, only the path prefixes will be measured.
func Collector(cfg Config, optPathPrefixFilters ...[]string) func(next http.Handler) http.Handler {
	if (!cfg.AllowAny && !cfg.AllowInternal && !cfg.CollectHttpRequestMetrics) && (cfg.Username == "" || cfg.Password == "") {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	pathPrefixFilters := []string{}
	if len(optPathPrefixFilters) > 0 {
		for _, v := range optPathPrefixFilters[0] {
			pathPrefixFilters = append(pathPrefixFilters, v)
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

			// check path filters
			if len(pathPrefixFilters) > 0 {
				found := false
				for _, p := range pathPrefixFilters {
					if strings.HasPrefix(r.URL.Path, p) {
						found = true
						break
					}
				}
				if !found && r.URL.Path != "/" {
					// skip measurement of the http request
					next.ServeHTTP(w, r)
					return
				}
			}

			// measure request
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			defer sample(time.Now().UTC(), r, ww)

			next.ServeHTTP(ww, r)
		})
	}
}
