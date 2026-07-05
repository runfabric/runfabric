package metrics

import (
	"runtime"
	"time"
)

// EnableRuntimeMetrics registers a scrape-time collector on reg (Default when
// nil) that exposes Go runtime/process gauges plus a build-info series. Safe to
// call more than once: the collector is keyed and re-registration replaces it.
//
// Exposed series:
//
//	runfabric_build_info{version,go_version} 1
//	runfabric_process_start_time_seconds
//	runfabric_process_uptime_seconds
//	runfabric_process_goroutines
//	runfabric_process_cpu_count
//	runfabric_process_memory_heap_alloc_bytes
//	runfabric_process_memory_sys_bytes
//	runfabric_process_gc_cycles
func EnableRuntimeMetrics(reg *Registry, version string) {
	if reg == nil {
		reg = Default
	}
	start := time.Now()
	if version == "" {
		version = "unknown"
	}
	buildLabels := map[string]string{"version": version, "go_version": runtime.Version()}

	reg.RegisterCollector("runtime", func() {
		reg.SetGauge("runfabric_build_info",
			"Build information; value is always 1.", buildLabels, 1)
		reg.SetGauge("runfabric_process_start_time_seconds",
			"Unix time the process started.", nil, float64(start.Unix()))
		reg.SetGauge("runfabric_process_uptime_seconds",
			"Seconds since the process started.", nil, time.Since(start).Seconds())
		reg.SetGauge("runfabric_process_goroutines",
			"Current number of goroutines.", nil, float64(runtime.NumGoroutine()))
		reg.SetGauge("runfabric_process_cpu_count",
			"Number of logical CPUs usable by the process.", nil, float64(runtime.NumCPU()))

		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		reg.SetGauge("runfabric_process_memory_heap_alloc_bytes",
			"Bytes of allocated heap objects.", nil, float64(ms.HeapAlloc))
		reg.SetGauge("runfabric_process_memory_sys_bytes",
			"Total bytes obtained from the OS.", nil, float64(ms.Sys))
		reg.SetGauge("runfabric_process_gc_cycles",
			"Completed GC cycles since process start.", nil, float64(ms.NumGC))
	})
}
