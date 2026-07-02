// Package metrics is a small, dependency-free Prometheus metrics registry and
// text-exposition exporter. It avoids pulling in the Prometheus client library
// while still producing output that any Prometheus/OpenMetrics scraper accepts.
//
// Supported metric types: Counter, Gauge, and Histogram, each with optional
// constant label sets. Use the package-level Default registry (exposed via
// Handler) or construct an isolated Registry for tests.
package metrics

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Default is the process-wide registry served by Handler.
var Default = NewRegistry()

// Registry holds a set of metric families and renders them in Prometheus text
// exposition format. It is safe for concurrent use.
type Registry struct {
	mu       sync.RWMutex
	families map[string]*family
}

type metricType string

const (
	typeCounter   metricType = "counter"
	typeGauge     metricType = "gauge"
	typeHistogram metricType = "histogram"
)

type family struct {
	name    string
	help    string
	typ     metricType
	buckets []float64 // histogram only
	series  map[string]*series
}

type series struct {
	labels       map[string]string
	value        float64  // counter/gauge
	bucketCounts []uint64 // histogram: cumulative count of observations <= buckets[i]
	sum          float64  // histogram
	count        uint64   // histogram
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{families: map[string]*family{}}
}

func (r *Registry) family(name, help string, typ metricType, buckets []float64) *family {
	r.mu.Lock()
	defer r.mu.Unlock()
	f, ok := r.families[name]
	if !ok {
		f = &family{name: name, help: help, typ: typ, buckets: buckets, series: map[string]*series{}}
		r.families[name] = f
	}
	return f
}

func seriesKey(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(labels[k])
	}
	return b.String()
}

func (f *family) seriesFor(labels map[string]string) *series {
	key := seriesKey(labels)
	s, ok := f.series[key]
	if !ok {
		s = &series{labels: labels}
		if f.typ == typeHistogram {
			s.bucketCounts = make([]uint64, len(f.buckets))
		}
		f.series[key] = s
	}
	return s
}

// IncCounter adds 1 to the named counter for the given labels.
func (r *Registry) IncCounter(name, help string, labels map[string]string) {
	r.AddCounter(name, help, labels, 1)
}

// AddCounter adds delta (>=0) to the named counter.
func (r *Registry) AddCounter(name, help string, labels map[string]string, delta float64) {
	if delta < 0 {
		return
	}
	f := r.family(name, help, typeCounter, nil)
	r.mu.Lock()
	f.seriesFor(labels).value += delta
	r.mu.Unlock()
}

// SetGauge sets the named gauge to v.
func (r *Registry) SetGauge(name, help string, labels map[string]string, v float64) {
	f := r.family(name, help, typeGauge, nil)
	r.mu.Lock()
	f.seriesFor(labels).value = v
	r.mu.Unlock()
}

// AddGauge adds delta (may be negative) to the named gauge.
func (r *Registry) AddGauge(name, help string, labels map[string]string, delta float64) {
	f := r.family(name, help, typeGauge, nil)
	r.mu.Lock()
	f.seriesFor(labels).value += delta
	r.mu.Unlock()
}

// Observe records v into the named histogram, creating it with buckets on first
// use. buckets must be sorted ascending; the +Inf bucket is implicit.
func (r *Registry) Observe(name, help string, buckets []float64, labels map[string]string, v float64) {
	f := r.family(name, help, typeHistogram, buckets)
	r.mu.Lock()
	s := f.seriesFor(labels)
	s.sum += v
	s.count++
	for i, ub := range f.buckets {
		if v <= ub {
			s.bucketCounts[i]++
		}
	}
	r.mu.Unlock()
}

// Handler returns an http.Handler that renders the registry on GET.
func (r *Registry) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = w.Write([]byte(r.Render()))
	})
}

// Handler renders the Default registry.
func Handler() http.Handler { return Default.Handler() }

// Render produces the Prometheus text exposition for the registry.
func (r *Registry) Render() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.families))
	for n := range r.families {
		names = append(names, n)
	}
	sort.Strings(names)

	var b strings.Builder
	for _, n := range names {
		f := r.families[n]
		if f.help != "" {
			fmt.Fprintf(&b, "# HELP %s %s\n", f.name, escapeHelp(f.help))
		}
		fmt.Fprintf(&b, "# TYPE %s %s\n", f.name, f.typ)

		keys := make([]string, 0, len(f.series))
		for k := range f.series {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			s := f.series[k]
			switch f.typ {
			case typeCounter, typeGauge:
				fmt.Fprintf(&b, "%s%s %s\n", f.name, renderLabels(s.labels, "", ""), formatFloat(s.value))
			case typeHistogram:
				// bucketCounts[i] already holds the cumulative count of
				// observations <= buckets[i] (maintained in Observe).
				for i, ub := range f.buckets {
					fmt.Fprintf(&b, "%s_bucket%s %d\n", f.name, renderLabels(s.labels, "le", formatFloat(ub)), s.bucketCounts[i])
				}
				fmt.Fprintf(&b, "%s_bucket%s %d\n", f.name, renderLabels(s.labels, "le", "+Inf"), s.count)
				fmt.Fprintf(&b, "%s_sum%s %s\n", f.name, renderLabels(s.labels, "", ""), formatFloat(s.sum))
				fmt.Fprintf(&b, "%s_count%s %d\n", f.name, renderLabels(s.labels, "", ""), s.count)
			}
		}
	}
	return b.String()
}

// renderLabels renders a label set, optionally adding one extra label
// (extraKey=extraVal, used for histogram "le").
func renderLabels(labels map[string]string, extraKey, extraVal string) string {
	if len(labels) == 0 && extraKey == "" {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys)+1)
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%q", k, escapeLabelValue(labels[k])))
	}
	if extraKey != "" {
		parts = append(parts, fmt.Sprintf("%s=%q", extraKey, extraVal))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}

func escapeHelp(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, "\n", `\n`)
}

func escapeLabelValue(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return strings.ReplaceAll(s, `"`, `\"`)
}
