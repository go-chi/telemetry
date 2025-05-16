package telemetry

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/uber-go/tally/v4"
	"github.com/uber-go/tally/v4/prometheus"
)

// reporter is the global prometheus reporter, which will also register
// the metrics with the prometheus registry. The Collector middleware will
// use this reporter to expose the metrics on the /metrics endpoint.
var reporter = prometheus.NewReporter(prometheus.Options{})

var defaultBucketsForIntegerValues = tally.ValueBuckets{
	1,
	2,
	5,
	7,
	9,
	10,
	50,
	100,
}

var defaultBucketFactorsForDurations = []float64{
	0.001,
	0.005,
	0.01,
	0.05,
	0.1,
	0.25,
	0.5,
	0.75,
	0.9,
	0.95,
	0.99,
	1,
	2.5,
	5,
	10,
	25,
	50,
	100,
}

// Scope represents the measurements scope for an application.
type Scope struct {
	scope  tally.Scope
	closer io.Closer
	cache  scopeCache
}

// NewScope creates a scope for an application. Receives a scope
// argument (a single-word) that is used as a prefix for all measurements.
// Optionally you can pass a map of tags to be used for all measurements in
// the scope.
func NewScope(scope string, optTags ...map[string]string) *Scope {
	var tags map[string]string = nil
	if len(optTags) > 0 {
		tags = optTags[0]
	}

	s, closer := newRootScope(tally.ScopeOptions{
		Prefix: scope,
		Tags:   tags,
	}, 1*time.Second)

	return &Scope{
		scope:  s,
		closer: closer,
		cache:  newScopeCache(),
	}
}

// WithTaggedMap returns a new scope with the given tags. However, if this is a
// hot-path (and telemetry usually is), we recommend using GetTaggedScope
// and SetTaggedScope to avoid the overhead of creating a new scope.
func (n *Scope) WithTaggedMap(tags map[string]string) *Scope {
	return &Scope{
		scope:  n.scope.Tagged(tags),
		closer: n.closer,
		cache:  newScopeCache(),
	}
}

// Tagged returns a tagged scope for the tag name and value pair. It's very
// efficient as it will return a cached scope if it exists for the tagged
// scope, or it will create a new scope and cache it based on tag name + value.
// If either tagName or tagValue is empty, it will return the current scope.
func (n *Scope) Tagged(tagName, tagValue string) *Scope {
	if tagName == "" || tagValue == "" {
		return n
	}
	metricsKey := fmt.Sprintf("%s:%s", tagName, tagValue)
	scope, _ := n.GetTaggedScope(metricsKey)
	if scope == nil {
		scope = n.SetTaggedScope(metricsKey, map[string]string{tagName: tagValue})
	}
	return scope
}

// GetTaggedScope returns a new scope with the given tags based on the
// supplied key identifier.
func (n *Scope) GetTaggedScope(key string) (*Scope, bool) {
	var scope *Scope
	var ok bool
	n.cache.taggedMu.RLock()
	scope, ok = n.cache.tagged[key]
	n.cache.taggedMu.RUnlock()
	return scope, ok
}

// SetTaggedScope returns a new scope with the given tags based on the
// supplied key identifier.
func (n *Scope) SetTaggedScope(key string, tags map[string]string) *Scope {
	s := &Scope{
		scope:  n.scope.Tagged(tags),
		closer: n.closer,
		cache:  newScopeCache(),
	}
	n.cache.taggedMu.Lock()
	n.cache.tagged[key] = s
	n.cache.taggedMu.Unlock()
	return s
}

// Close closes the scope and stops reporting.
func (n *Scope) Close() error {
	return n.closer.Close()
}

// RecordHit increases a hit counter. This is ideal for counting HTTP requests
// or other events that are incremented by one each time.
//
// RecordHit adds the "_total" suffix to the name of the measurement.
//
// Measurement type: Counter
func (n *Scope) RecordHit(measurement string) {
	n.RecordIncrementValue(measurement, 1)
}

// RecordIncrementValue increases a counter.
//
// RecordIncrementValue adds the "_total" suffix to the name of the measurement.
//
// Measurement type: Counter
func (n *Scope) RecordIncrementValue(measurement string, value int64) {
	var scope tally.Counter
	var ok bool
	n.cache.counterMu.RLock()
	scope, ok = n.cache.counter[measurement]
	n.cache.counterMu.RUnlock()
	if !ok {
		n.cache.counterMu.Lock()
		scope = n.scope.Counter(SnakeCasef(measurement + "_total"))
		n.cache.counter[measurement] = scope
		n.cache.counterMu.Unlock()
	}
	scope.Inc(value)
}

// RecordGauge sets the value of a measurement that can go up or down over
// time.
//
// RecordGauge measures a prometheus raw type and no suffix is added to the
// measurement.
//
// Measurement type: Gauge
func (n *Scope) RecordGauge(measurement string, value float64) {
	var scope tally.Gauge
	var ok bool
	n.cache.gaugeMu.RLock()
	scope, ok = n.cache.gauge[measurement]
	n.cache.gaugeMu.RUnlock()
	if !ok {
		n.cache.gaugeMu.Lock()
		scope = n.scope.Gauge(SnakeCasef(measurement))
		n.cache.gauge[measurement] = scope
		n.cache.gaugeMu.Unlock()
	}
	scope.Update(value)
}

// RecordSize records a numeric unit-less value that can go up or down. Use it
// when it's more important to know the last value of said size. This is useful
// to measure things like the size of a queue.
//
// RecordSize adds the "_size" prefix to the name of the measurement.
//
// Measurement type: Gauge
func (n *Scope) RecordSize(measurement string, value float64) {
	n.RecordGauge(measurement+"_size", value)
}

// RecordIntegerValue records a numeric unit-less value that can go up or down.
// Use it when is important to see how the value evolved over time.
//
// RecordIntegerValue uses an histogram configured with buckets that priorize
// values closer to zero.
//
// Measurement type: Histogram
func (n *Scope) RecordIntegerValue(measurement string, value int) {
	n.RecordValueWithBuckets(measurement, float64(value), defaultBucketsForIntegerValues)
}

// RecordValue records a numeric unit-less value that can go up or down. Use it
// when is important to see how the value evolved over time.
//
// RecordValue measures a prometheus raw type and no suffix is added to the measurement.
//
// Measurement type: Histogram
func (n *Scope) RecordValue(measurement string, value float64) {
	n.RecordValueWithBuckets(measurement, value, nil)
}

// RecordValueWithBuckets records a numeric unit-less value that can go up or
// down. Use it when is important to see how the value evolved over time.
//
// RecordValueWithBuckets adds the "_value" suffix to the name of the measurement.
//
// Measurement type: Histogram
func (n *Scope) RecordValueWithBuckets(measurement string, value float64, buckets []float64) {
	var scope tally.Histogram
	var ok bool
	n.cache.histogramMu.RLock()
	scope, ok = n.cache.histogram[measurement]
	n.cache.histogramMu.RUnlock()
	if !ok {
		n.cache.histogramMu.Lock()
		scope = n.scope.Histogram(SnakeCasef(measurement+"_value"), tally.ValueBuckets(buckets))
		n.cache.histogram[measurement] = scope
		n.cache.histogramMu.Unlock()
	}
	scope.RecordValue(value)
}

// RecordDuration records an elapsed time. Use it when is important to see how
// values. Use it when is important to see how a value evolved over time, for
// instance request durations, the time it takes for a task to finish, etc.
//
// RecordDuration adds the "_duration_seconds" prefix to the name of the
// measurement.
//
// Measurement type: Histogram
func (n *Scope) RecordDuration(measurement string, start time.Time, stop time.Time) {
	n.RecordDurationWithResolution(measurement, start, stop, 0)
}

// RecordDurationWithResolution records the elapsed duration between two time
// values. Use it when is important to see how a value evolved over time, for
// instance request durations, the time it takes for a task to finish, etc.
//
// The resolution parameter can be any value, this value will be taken as base
// to build buckets.
//
// RecordDurationWithResolution adds the "_duration_seconds" prefix to the name
// of the measurement.
//
// Measurement type: Histogram
func (n *Scope) RecordDurationWithResolution(measurement string, timeA time.Time, timeB time.Time, resolution time.Duration) {
	if resolution <= 0 {
		resolution = time.Second
	}

	var buckets tally.Buckets
	var ok bool
	resKey := int64(resolution)

	n.cache.bucketsMu.RLock()
	buckets, ok = n.cache.buckets[resKey]
	n.cache.bucketsMu.RUnlock()

	if !ok {
		unit := float64(resolution)
		durations := make([]time.Duration, len(defaultBucketFactorsForDurations))
		for i := range durations {
			durations[i] = time.Duration(int64(unit * defaultBucketFactorsForDurations[i]))
		}
		calculatedBuckets := tally.DurationBuckets(durations)

		n.cache.bucketsMu.Lock()
		buckets = calculatedBuckets
		n.cache.buckets[resKey] = buckets
		n.cache.bucketsMu.Unlock()
	}

	var scope tally.Histogram
	metricsKey := fmt.Sprintf("%s:%d", measurement, resKey)

	n.cache.histogramMu.RLock()
	scope, ok = n.cache.histogram[metricsKey]
	n.cache.histogramMu.RUnlock()
	if !ok {
		n.cache.histogramMu.Lock()
		scope = n.scope.Histogram(SnakeCasef(measurement+"_duration_seconds"), buckets)
		n.cache.histogram[metricsKey] = scope
		n.cache.histogramMu.Unlock()
	}

	elapsed := timeB.Sub(timeA)
	if elapsed < 0 {
		elapsed = elapsed * -1
	}
	scope.RecordDuration(elapsed)
}

// RecordSpan starts a stopwatch for a span.
//
// RecordSpan adds the "_span" suffix to the name of the measurement.
//
// Measurement type: Timer
func (n *Scope) RecordSpan(measurement string) tally.Stopwatch {
	var scope tally.Timer
	var ok bool
	n.cache.timerMu.RLock()
	scope, ok = n.cache.timer[measurement]
	n.cache.timerMu.RUnlock()
	if !ok {
		n.cache.timerMu.Lock()
		scope = n.scope.Timer(SnakeCasef(measurement + "_span"))
		n.cache.timer[measurement] = scope
		n.cache.timerMu.Unlock()
	}
	return scope.Start()
}

func SnakeCasef(format string, args ...any) string {
	s := strings.ToLower(fmt.Sprintf(format, args...))
	if strings.Contains(s, "-") {
		s = strings.ReplaceAll(s, "-", "_")
	}
	if strings.Contains(s, " ") {
		s = strings.ReplaceAll(s, " ", "_")
	}
	if strings.Contains(s, ".") {
		s = strings.ReplaceAll(s, ".", "_")
	}
	return s
}

func newRootScope(opts tally.ScopeOptions, interval time.Duration) (tally.Scope, io.Closer) {
	opts.CachedReporter = reporter
	opts.Separator = prometheus.DefaultSeparator
	opts.SanitizeOptions = nil // noop sanitizer
	opts.OmitCardinalityMetrics = true
	return tally.NewRootScope(opts, interval)
}

type scopeCache struct {
	tagged      map[string]*Scope // map key is a unique identifier for the group of tags
	taggedMu    sync.RWMutex
	counter     map[string]tally.Counter
	counterMu   sync.RWMutex
	gauge       map[string]tally.Gauge
	gaugeMu     sync.RWMutex
	timer       map[string]tally.Timer
	timerMu     sync.RWMutex
	histogram   map[string]tally.Histogram
	histogramMu sync.RWMutex
	buckets     map[int64]tally.Buckets
	bucketsMu   sync.RWMutex
}

func newScopeCache() scopeCache {
	return scopeCache{
		tagged:    make(map[string]*Scope),
		counter:   make(map[string]tally.Counter),
		gauge:     make(map[string]tally.Gauge),
		timer:     make(map[string]tally.Timer),
		histogram: make(map[string]tally.Histogram),
		buckets:   make(map[int64]tally.Buckets),
	}
}
