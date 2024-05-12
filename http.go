package telemetry

import (
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

var httpMetrics = NewScope("http")

func sample(start time.Time, asteriskAltenative string, r *http.Request, ww middleware.WrapResponseWriter) {
	pattern := chi.RouteContext(r.Context()).RoutePattern()
	if pattern == "" {
		return
	}

	pattern = strings.Replace(pattern, "*", asteriskAltenative, -1) // Prometheus does not allow * in metric label

	status := ww.Status()
	if status == 0 { // TODO: see why we have status = 0 under test conditions (this came up during benchmarks for some of the requests)
		status = http.StatusOK
	}
	// Prometheus errors if string is not uft8 encoded
	if !utf8.ValidString(pattern) {
		return
	}
	labels := map[string]string{
		"endpoint": fmt.Sprintf("%s %s", strings.ToLower(r.Method), pattern),
		"status":   fmt.Sprintf("%d", status),
	}

	httpMetrics.RecordDuration("request", labels, start, time.Now().UTC())
	httpMetrics.RecordHit("requests", labels)
}
