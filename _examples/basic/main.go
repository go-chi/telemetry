package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/telemetry"
)

var (
	AppMetrics = &MyAppMetrics{telemetry.NewScope("app")}
)

type MyAppMetrics struct {
	*telemetry.Scope
}

func (m *MyAppMetrics) RecordMyAppHit() {
	m.RecordHit("my_app_hit", nil)
}

func (m *MyAppMetrics) RecordAppGauge(value float64) {
	m.RecordGauge("my_app_gauge", nil, value)
}

func main() {
	r := chi.NewRouter()

	r.Use(middleware.Logger)

	// telemetry.Collector middleware mounts /metrics endpoint
	// with prometheus metrics collector.
	r.Use(telemetry.Collector(telemetry.Config{
		AllowAny: true,
	}, []string{"/api"})) // path prefix filters records generic http request metrics

	r.Route("/api", func(r chi.Router) {
		r.Get("/hello", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("Hello World!"))
		})
		r.Get("/hit", func(w http.ResponseWriter, r *http.Request) {
			// record a hit
			AppMetrics.RecordMyAppHit()
			w.Write([]byte("Hit recorded!"))
		})
		r.Get("/gauge/{value}", func(w http.ResponseWriter, r *http.Request) {
			value := chi.URLParam(r, "value")
			floatValue, err := strconv.ParseFloat(value, 64)
			if err != nil {
				w.Write([]byte("Invalid value"))
				return
			}
			// record a gauge
			AppMetrics.RecordAppGauge(floatValue)
			w.Write([]byte("Gauge recorded!"))
		})
	})

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	srvAddr := "localhost:3000"
	srv := &http.Server{
		Addr:    srvAddr,
		Handler: r,
	}

	go func() {
		<-sig

		slog.Info("server is shutting down")

		err := srv.Shutdown(context.Background())
		if err != nil {
			panic(err)
		}
	}()

	slog.Info("server is running on", "address", srvAddr)

	err := srv.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}
