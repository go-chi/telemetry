package telemetry

import (
	"fmt"
	"io"
	"time"

	"github.com/uber-go/tally/v4"
	"github.com/uber-go/tally/v4/prometheus"
)

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
}

// NewScope creates a scope for an application. Receives a scope
// argument (a single-word) that is used as a prefix for all measurements.
func NewScope(scope string) *Scope {
	s, closer := newRootScope(tally.ScopeOptions{
		Prefix: scope,
	}, 1*time.Second)
	return &Scope{
		scope:  s,
		closer: closer,
	}
}

// Close closes the scope and stops reporting.
func (n *Scope) Close() error {
	return n.closer.Close()
}

// RecordHit increases a hit counter. This is ideal for counting HTTP requests
// or other events that are incremented by one each time.
//
// RecordHit adds the "_total" suffix to the name of the measurement.
func (n *Scope) RecordHit(measurement string, tags map[string]string) {
	record := n.scope.Tagged(tags).Counter(fmt.Sprintf(measurement + "_total"))
	record.Inc(1.0)
}

func (n *Scope) RecordIncrementValue(measurement string, tags map[string]string, value int64) {
	record := n.scope.Tagged(tags).Counter(fmt.Sprintf(measurement + "_total"))
	record.Inc(value)
}

// RecordGauge sets the value of a measurement that can go up or down over
// time.
//
// RecordGauge measures a prometheus raw type and no suffix is added to the
// measurement.
func (n *Scope) RecordGauge(measurement string, tags map[string]string, value float64) {
	record := n.scope.Tagged(tags).Gauge(measurement)
	record.Update(value)
}

// RecordSize records a numeric unit-less value that can go up or down. Use it
// when it's more important to know the last value of said size. This is useful
// to measure things like the size of a queue.
//
// RecordSize adds the "_size" prefix to the name of the measurement.
func (n *Scope) RecordSize(measurement string, tags map[string]string, value float64) {
	n.RecordGauge(fmt.Sprintf(measurement+"_size"), tags, value)
}

// RecordIntegerValue records a numeric unit-less value that can go up or down.
// Use it when is important to see how the value evolved over time.
//
// RecordIntegerValue uses an histogram configured with buckets that priorize
// values closer to zero.
func (n *Scope) RecordIntegerValue(measurement string, tags map[string]string, value int) {
	record := n.scope.Tagged(tags).
		Histogram(fmt.Sprintf(measurement+"_value"), defaultBucketsForIntegerValues)
	record.RecordValue(float64(value))
}

// RecordValue records a numeric unit-less value that can go up or down. Use it
// when is important to see how the value evolved over time.
//
// RecordValue measures a prometheus raw type and no suffix is added to the measurement.
func (n *Scope) RecordValue(measurement string, tags map[string]string, value float64) {
	n.RecordValueWithBuckets(measurement, tags, value, nil)
}

// RecordValueWithBuckets records a numeric unit-less value that can go up or
// down. Use it when is important to see how the value evolved over time.
//
// RecordValueWithBuckets adds the "_value" suffix to the name of the measurement.
func (n *Scope) RecordValueWithBuckets(measurement string, tags map[string]string, value float64, buckets []float64) {
	record := n.scope.Tagged(tags).
		Histogram(fmt.Sprintf(measurement+"_value"), tally.ValueBuckets(buckets))
	record.RecordValue(value)
}

// RecordDuration records an elapsed time. Use it when is important to see how
// values. Use it when is important to see how a value evolved over time, for
// instance request durations, the time it takes for a task to finish, etc.
//
// RecordDuration adds the "_duration_seconds" prefix to the name of the
// measurement.
func (n *Scope) RecordDuration(measurement string, tags map[string]string, start time.Time, stop time.Time) {
	n.RecordDurationWithResolution(measurement, tags, start, stop, 0)
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
func (n *Scope) RecordDurationWithResolution(measurement string, tags map[string]string, timeA time.Time, timeB time.Time, resolution time.Duration) {
	var buckets tally.Buckets

	if resolution <= 0 {
		resolution = time.Second
	}

	unit := float64(resolution)
	durations := make([]time.Duration, len(defaultBucketFactorsForDurations))
	for i := range durations {
		durations[i] = time.Duration(int64(unit * defaultBucketFactorsForDurations[i]))
	}
	buckets = tally.DurationBuckets(durations)

	record := n.scope.Tagged(tags).Histogram(
		fmt.Sprintf(measurement+"_duration_seconds"),
		buckets,
	)
	elapsed := timeB.Sub(timeA)
	if elapsed < 0 {
		elapsed = elapsed * -1
	}
	record.RecordDuration(elapsed)
}

func newRootScope(opts tally.ScopeOptions, interval time.Duration) (tally.Scope, io.Closer) {
	opts.CachedReporter = reporter
	opts.Separator = prometheus.DefaultSeparator
	opts.SanitizeOptions = &prometheus.DefaultSanitizerOpts
	return tally.NewRootScope(opts, interval)
}
