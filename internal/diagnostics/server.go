package diagnostics

import (
	"encoding/json"
	"expvar"
	"math"
	"net/http"
	"net/http/pprof"
	"runtime"
	"runtime/metrics"
	"time"

	"github.com/Joaopdiasventura/rinha-de-backend-2026/internal/perf"
)

var metricNames = []string{
	"/cpu/classes/gc/mark/assist:cpu-seconds",
	"/gc/cycles/automatic:gc-cycles",
	"/gc/heap/allocs:bytes",
	"/gc/heap/frees:bytes",
	"/gc/heap/live:bytes",
	"/gc/heap/objects:objects",
	"/gc/pauses:seconds",
	"/memory/classes/heap/free:bytes",
	"/memory/classes/heap/objects:bytes",
	"/memory/classes/heap/released:bytes",
	"/memory/classes/heap/stacks:bytes",
	"/memory/classes/metadata/mspan/free:bytes",
	"/memory/classes/metadata/mspan/inuse:bytes",
	"/memory/classes/metadata/mcache/free:bytes",
	"/memory/classes/metadata/mcache/inuse:bytes",
	"/memory/classes/total:bytes",
	"/sched/gomaxprocs:threads",
	"/sched/goroutines:goroutines",
}

type runtimeSnapshot struct {
	Timestamp         string                 `json:"timestamp"`
	NumCPU            int                    `json:"num_cpu"`
	GOMAXPROCS        int                    `json:"gomaxprocs"`
	NumGoroutine      int                    `json:"num_goroutine"`
	MemStats          memStatsSnapshot       `json:"mem_stats"`
	RuntimeMetricData map[string]interface{} `json:"runtime_metrics"`
	AppMetrics        perf.Snapshot          `json:"app_metrics"`
}

type memStatsSnapshot struct {
	AllocBytes      uint64 `json:"alloc_bytes"`
	TotalAllocBytes uint64 `json:"total_alloc_bytes"`
	SysBytes        uint64 `json:"sys_bytes"`
	HeapAllocBytes  uint64 `json:"heap_alloc_bytes"`
	HeapInuseBytes  uint64 `json:"heap_inuse_bytes"`
	HeapIdleBytes   uint64 `json:"heap_idle_bytes"`
	HeapReleased    uint64 `json:"heap_released_bytes"`
	HeapObjects     uint64 `json:"heap_objects"`
	StackInuseBytes uint64 `json:"stack_inuse_bytes"`
	NumGC           uint32 `json:"num_gc"`
	PauseTotalNs    uint64 `json:"pause_total_ns"`
}

type histogramSnapshot struct {
	Buckets []float64 `json:"buckets"`
	Counts  []uint64  `json:"counts"`
}

func NewHandler() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/debug/vars", expvar.Handler())
	mux.HandleFunc("/debug/runtime/metrics", runtimeMetricsHandler)
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	return mux
}

func runtimeMetricsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	samples := make([]metrics.Sample, len(metricNames))
	for i, name := range metricNames {
		samples[i].Name = name
	}

	metrics.Read(samples)

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	payload := runtimeSnapshot{
		Timestamp:    time.Now().UTC().Format(time.RFC3339Nano),
		NumCPU:       runtime.NumCPU(),
		GOMAXPROCS:   runtime.GOMAXPROCS(0),
		NumGoroutine: runtime.NumGoroutine(),
		MemStats: memStatsSnapshot{
			AllocBytes:      mem.Alloc,
			TotalAllocBytes: mem.TotalAlloc,
			SysBytes:        mem.Sys,
			HeapAllocBytes:  mem.HeapAlloc,
			HeapInuseBytes:  mem.HeapInuse,
			HeapIdleBytes:   mem.HeapIdle,
			HeapReleased:    mem.HeapReleased,
			HeapObjects:     mem.HeapObjects,
			StackInuseBytes: mem.StackInuse,
			NumGC:           mem.NumGC,
			PauseTotalNs:    mem.PauseTotalNs,
		},
		RuntimeMetricData: map[string]interface{}{},
		AppMetrics:        perf.SnapshotState(),
	}

	for _, sample := range samples {
		payload.RuntimeMetricData[sample.Name] = sampleValue(sample.Value)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func sampleValue(value metrics.Value) interface{} {
	switch value.Kind() {
	case metrics.KindBad:
		return nil
	case metrics.KindUint64:
		return value.Uint64()
	case metrics.KindFloat64:
		return value.Float64()
	case metrics.KindFloat64Histogram:
		histogram := value.Float64Histogram()
		return histogramSnapshot{
			Buckets: sanitizeFloat64Slice(histogram.Buckets),
			Counts:  histogram.Counts,
		}
	default:
		return nil
	}
}

func sanitizeFloat64Slice(values []float64) []float64 {
	sanitized := make([]float64, len(values))

	for i, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			sanitized[i] = 0
			continue
		}

		sanitized[i] = value
	}

	return sanitized
}
