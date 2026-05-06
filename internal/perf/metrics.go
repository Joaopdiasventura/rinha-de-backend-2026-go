package perf

import (
	"sync/atomic"
	"time"
)

var enabled atomic.Bool

var startedAt = time.Now()

var requests atomic.Uint64
var decodeErrors atomic.Uint64
var serviceErrors atomic.Uint64
var encodeErrors atomic.Uint64

var decodeNanos atomic.Uint64
var vectorizeNanos atomic.Uint64
var searchNanos atomic.Uint64
var encodeNanos atomic.Uint64
var totalNanos atomic.Uint64

var searchVisited atomic.Uint64
var searchPrunedStage1 atomic.Uint64
var searchPrunedStage2 atomic.Uint64
var searchPrunedStage3 atomic.Uint64
var searchInserted atomic.Uint64
var searchStride atomic.Uint64

type Snapshot struct {
	Enabled             bool    `json:"enabled"`
	UptimeSeconds       float64 `json:"uptime_seconds"`
	Requests            uint64  `json:"requests"`
	RequestsPerSecond   float64 `json:"requests_per_second_avg"`
	DecodeErrors        uint64  `json:"decode_errors"`
	ServiceErrors       uint64  `json:"service_errors"`
	EncodeErrors        uint64  `json:"encode_errors"`
	AvgDecodeMs         float64 `json:"avg_decode_ms"`
	AvgVectorizeMs      float64 `json:"avg_vectorize_ms"`
	AvgSearchMs         float64 `json:"avg_search_ms"`
	AvgEncodeMs         float64 `json:"avg_encode_ms"`
	AvgTotalMs          float64 `json:"avg_total_ms"`
	SearchVisited       uint64  `json:"search_visited"`
	SearchInserted      uint64  `json:"search_inserted"`
	SearchPrunedStage1  uint64  `json:"search_pruned_stage1"`
	SearchPrunedStage2  uint64  `json:"search_pruned_stage2"`
	SearchPrunedStage3  uint64  `json:"search_pruned_stage3"`
	SearchStride        uint64  `json:"search_stride"`
	AvgVisitedPerSearch float64 `json:"avg_visited_per_search"`
	PruneRate           float64 `json:"prune_rate"`
}

func Configure(value bool) {
	enabled.Store(value)
}

func Enabled() bool {
	return enabled.Load()
}

func ObserveDecodeError() {
	if !Enabled() {
		return
	}

	decodeErrors.Add(1)
}

func ObserveServiceError() {
	if !Enabled() {
		return
	}

	serviceErrors.Add(1)
}

func ObserveEncodeError() {
	if !Enabled() {
		return
	}

	encodeErrors.Add(1)
}

func ObserveRequest(decode, vectorize, search, encode, total time.Duration) {
	if !Enabled() {
		return
	}

	requests.Add(1)
	decodeNanos.Add(uint64(decode))
	vectorizeNanos.Add(uint64(vectorize))
	searchNanos.Add(uint64(search))
	encodeNanos.Add(uint64(encode))
	totalNanos.Add(uint64(total))
}

func ObserveSearch(visited, prunedStage1, prunedStage2, prunedStage3, inserted, stride int) {
	if !Enabled() {
		return
	}

	searchVisited.Add(uint64(visited))
	searchPrunedStage1.Add(uint64(prunedStage1))
	searchPrunedStage2.Add(uint64(prunedStage2))
	searchPrunedStage3.Add(uint64(prunedStage3))
	searchInserted.Add(uint64(inserted))
	searchStride.Store(uint64(stride))
}

func SnapshotState() Snapshot {
	requestCount := requests.Load()
	uptimeSeconds := time.Since(startedAt).Seconds()

	visited := searchVisited.Load()
	pruned1 := searchPrunedStage1.Load()
	pruned2 := searchPrunedStage2.Load()
	pruned3 := searchPrunedStage3.Load()
	totalPruned := pruned1 + pruned2 + pruned3

	snapshot := Snapshot{
		Enabled:            Enabled(),
		UptimeSeconds:      uptimeSeconds,
		Requests:           requestCount,
		DecodeErrors:       decodeErrors.Load(),
		ServiceErrors:      serviceErrors.Load(),
		EncodeErrors:       encodeErrors.Load(),
		SearchVisited:      visited,
		SearchInserted:     searchInserted.Load(),
		SearchPrunedStage1: pruned1,
		SearchPrunedStage2: pruned2,
		SearchPrunedStage3: pruned3,
		SearchStride:       searchStride.Load(),
	}

	if uptimeSeconds > 0 {
		snapshot.RequestsPerSecond = float64(requestCount) / uptimeSeconds
	}

	if requestCount > 0 {
		snapshot.AvgDecodeMs = nanosToMilliseconds(decodeNanos.Load(), requestCount)
		snapshot.AvgVectorizeMs = nanosToMilliseconds(vectorizeNanos.Load(), requestCount)
		snapshot.AvgSearchMs = nanosToMilliseconds(searchNanos.Load(), requestCount)
		snapshot.AvgEncodeMs = nanosToMilliseconds(encodeNanos.Load(), requestCount)
		snapshot.AvgTotalMs = nanosToMilliseconds(totalNanos.Load(), requestCount)
		snapshot.AvgVisitedPerSearch = float64(visited) / float64(requestCount)
	}

	if visited > 0 {
		snapshot.PruneRate = float64(totalPruned) / float64(visited)
	}

	return snapshot
}

func nanosToMilliseconds(total uint64, count uint64) float64 {
	return float64(total) / float64(count) / float64(time.Millisecond)
}
