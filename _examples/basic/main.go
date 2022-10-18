package main

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/goware/telemetry"
)

var (
	AppMetrics = &MyAppMetrics{telemetry.NewNamespace("app")}
)

type MyAppMetrics struct {
	*telemetry.Namespace
}

func (m *MyAppMetrics) RecordMyAppHit() {
	m.RecordHit("my_app_hit", nil)
}

func (m *MyAppMetrics) RecordAppGuage(value float64) {
	m.RecordGauge("my_app_gauge", nil, value)
}

func main() {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(telemetry.Collector(telemetry.Config{
		AllowAny: true,
	}, []string{"/api"})) // path prefix filters basically records generic http request metrics

	r.Route("/api", func(r chi.Router) {
		r.Get("/hello", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("Hello World!"))
		})
		r.Get("/hit", func(w http.ResponseWriter, r *http.Request) {
			// record a hit
			AppMetrics.RecordMyAppHit()
			w.Write([]byte("Hit recorded!"))
		})
		r.Get("/guage/{value}", func(w http.ResponseWriter, r *http.Request) {
			value := chi.URLParam(r, "value")
			floatValue, err := strconv.ParseFloat(value, 64)
			if err != nil {
				w.Write([]byte("Invalid value"))
				return
			}
			// record a guage
			AppMetrics.RecordAppGuage(floatValue)
			w.Write([]byte("Guage recorded!"))
		})
	})

	http.ListenAndServe("localhost:3000", r)
}
