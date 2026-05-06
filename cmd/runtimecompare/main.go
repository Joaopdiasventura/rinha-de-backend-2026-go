package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
)

type runtimeSnapshot struct {
	NumGoroutine int `json:"num_goroutine"`
	MemStats     struct {
		HeapAllocBytes uint64 `json:"heap_alloc_bytes"`
	} `json:"mem_stats"`
}

func main() {
	beforePath := flag.String("before", "", "path to runtime metrics snapshot before the test")
	afterPath := flag.String("after", "", "path to runtime metrics snapshot after the test")
	maxGoroutineGrowth := flag.Float64("max-goroutine-growth", 10, "maximum allowed goroutine growth percentage")
	maxHeapGrowth := flag.Float64("max-heap-growth", 15, "maximum allowed heap growth percentage")
	flag.Parse()

	if *beforePath == "" || *afterPath == "" {
		fmt.Fprintln(os.Stderr, "missing -before or -after")
		os.Exit(2)
	}

	before, err := readSnapshot(*beforePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read before snapshot: %v\n", err)
		os.Exit(1)
	}

	after, err := readSnapshot(*afterPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read after snapshot: %v\n", err)
		os.Exit(1)
	}

	goroutineGrowth := percentageGrowth(float64(before.NumGoroutine), float64(after.NumGoroutine))
	heapGrowth := percentageGrowth(float64(before.MemStats.HeapAllocBytes), float64(after.MemStats.HeapAllocBytes))

	fmt.Println("runtime comparison:")
	fmt.Printf("  goroutines before: %d\n", before.NumGoroutine)
	fmt.Printf("  goroutines after:  %d\n", after.NumGoroutine)
	fmt.Printf("  goroutine growth:  %.2f%%\n", goroutineGrowth)
	fmt.Printf("  heap before:       %d bytes\n", before.MemStats.HeapAllocBytes)
	fmt.Printf("  heap after:        %d bytes\n", after.MemStats.HeapAllocBytes)
	fmt.Printf("  heap growth:       %.2f%%\n", heapGrowth)

	if goroutineGrowth > *maxGoroutineGrowth || heapGrowth > *maxHeapGrowth {
		fmt.Fprintln(os.Stderr, "runtime comparison failure reasons:")
		if goroutineGrowth > *maxGoroutineGrowth {
			fmt.Fprintf(os.Stderr, "  - goroutine growth exceeded limit: %.2f%% > %.2f%%\n", goroutineGrowth, *maxGoroutineGrowth)
		}
		if heapGrowth > *maxHeapGrowth {
			fmt.Fprintf(os.Stderr, "  - heap growth exceeded limit: %.2f%% > %.2f%%\n", heapGrowth, *maxHeapGrowth)
		}
		os.Exit(1)
	}
}

func readSnapshot(path string) (runtimeSnapshot, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return runtimeSnapshot{}, err
	}

	var snapshot runtimeSnapshot
	if err := json.Unmarshal(content, &snapshot); err != nil {
		return runtimeSnapshot{}, err
	}

	return snapshot, nil
}

func percentageGrowth(before float64, after float64) float64 {
	if before <= 0 {
		if after <= 0 {
			return 0
		}

		return math.Inf(1)
	}

	return ((after - before) / before) * 100
}
