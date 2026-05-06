package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

type summaryFile struct {
	Metrics map[string]json.RawMessage `json:"metrics"`
}

func main() {
	file := flag.String("file", "", "path to k6 summary export")
	flag.Parse()

	if *file == "" {
		fmt.Fprintln(os.Stderr, "missing -file")
		os.Exit(2)
	}

	content, err := os.ReadFile(*file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read summary: %v\n", err)
		os.Exit(1)
	}

	var summary summaryFile
	if err := json.Unmarshal(content, &summary); err != nil {
		fmt.Fprintf(os.Stderr, "parse summary: %v\n", err)
		os.Exit(1)
	}

	duration := metricValues(summary.Metrics["http_req_duration"])
	waiting := metricValues(summary.Metrics["http_req_waiting"])
	requests := metricValues(summary.Metrics["http_reqs"])
	failed := metricValues(summary.Metrics["http_req_failed"])

	fmt.Println("load test summary:")
	fmt.Printf("  p50:         %.2fms\n", value(duration, "med"))
	fmt.Printf("  p95:         %.2fms\n", value(duration, "p(95)"))
	fmt.Printf("  p99:         %.2fms\n", value(duration, "p(99)"))
	fmt.Printf("  req/s:       %.2f\n", value(requests, "rate"))
	fmt.Printf("  error rate:  %.4f\n", rateValue(failed))
	fmt.Printf("  waiting p99: %.2fms\n", value(waiting, "p(99)"))
	fmt.Printf("  requests:    %.0f\n", value(requests, "count"))
}

func metricValues(raw json.RawMessage) map[string]float64 {
	if len(raw) == 0 {
		return map[string]float64{}
	}

	var metric map[string]interface{}
	if err := json.Unmarshal(raw, &metric); err != nil {
		return map[string]float64{}
	}

	values := make(map[string]float64, len(metric))
	for key, value := range metric {
		if number, ok := value.(float64); ok {
			values[key] = number
		}
	}

	if nestedValues, ok := metric["values"].(map[string]interface{}); ok {
		for key, value := range nestedValues {
			if number, ok := value.(float64); ok {
				values[key] = number
			}
		}
	}

	return values
}

func rateValue(values map[string]float64) float64 {
	if rate, exists := values["rate"]; exists {
		return rate
	}

	if value, exists := values["value"]; exists {
		return value
	}

	return 0
}

func value(values map[string]float64, key string) float64 {
	if current, exists := values[key]; exists {
		return current
	}

	return 0
}
