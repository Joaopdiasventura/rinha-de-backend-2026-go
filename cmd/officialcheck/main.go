package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

type resultsFile struct {
	P99     string `json:"p99"`
	Scoring struct {
		Breakdown struct {
			FalsePositiveDetections int `json:"false_positive_detections"`
			FalseNegativeDetections int `json:"false_negative_detections"`
			TruePositiveDetections  int `json:"true_positive_detections"`
			TrueNegativeDetections  int `json:"true_negative_detections"`
			HTTPErrors              int `json:"http_errors"`
		} `json:"breakdown"`
		FailureRate string `json:"failure_rate"`
		P99Score    struct {
			Value        float64 `json:"value"`
			CutTriggered bool    `json:"cut_triggered"`
		} `json:"p99_score"`
		DetectionScore struct {
			Value         float64  `json:"value"`
			CutTriggered  bool     `json:"cut_triggered"`
			RateComponent *float64 `json:"rate_component"`
		} `json:"detection_score"`
		FinalScore float64 `json:"final_score"`
	} `json:"scoring"`
}

func main() {
	file := flag.String("file", "", "path to official results.json")
	flag.Parse()

	if *file == "" {
		fmt.Fprintln(os.Stderr, "missing -file")
		os.Exit(2)
	}

	content, err := os.ReadFile(*file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read results: %v\n", err)
		os.Exit(1)
	}

	var results resultsFile
	if err := json.Unmarshal(content, &results); err != nil {
		fmt.Fprintf(os.Stderr, "parse results: %v\n", err)
		os.Exit(1)
	}

	breakdown := results.Scoring.Breakdown
	fmt.Println("official test summary:")
	fmt.Printf("  p99:          %s\n", results.P99)
	fmt.Printf("  failure rate: %s\n", results.Scoring.FailureRate)
	fmt.Printf("  final score:  %.2f\n", results.Scoring.FinalScore)
	fmt.Printf("  true positives:  %d\n", breakdown.TruePositiveDetections)
	fmt.Printf("  true negatives:  %d\n", breakdown.TrueNegativeDetections)
	fmt.Printf("  false positives: %d\n", breakdown.FalsePositiveDetections)
	fmt.Printf("  false negatives: %d\n", breakdown.FalseNegativeDetections)
	fmt.Printf("  http errors:     %d\n", breakdown.HTTPErrors)

	if breakdown.FalsePositiveDetections != 0 ||
		breakdown.FalseNegativeDetections != 0 ||
		breakdown.HTTPErrors != 0 ||
		results.Scoring.P99Score.CutTriggered ||
		results.Scoring.DetectionScore.CutTriggered {
		fmt.Fprintln(os.Stderr, "official test failure reasons:")
		if breakdown.FalsePositiveDetections != 0 {
			fmt.Fprintf(os.Stderr, "  - false positives detected: %d\n", breakdown.FalsePositiveDetections)
		}
		if breakdown.FalseNegativeDetections != 0 {
			fmt.Fprintf(os.Stderr, "  - false negatives detected: %d\n", breakdown.FalseNegativeDetections)
		}
		if breakdown.HTTPErrors != 0 {
			fmt.Fprintf(os.Stderr, "  - HTTP errors detected: %d\n", breakdown.HTTPErrors)
		}
		if results.Scoring.P99Score.CutTriggered {
			fmt.Fprintf(os.Stderr, "  - p99 cut triggered at %s\n", results.P99)
		}
		if results.Scoring.DetectionScore.CutTriggered {
			fmt.Fprintln(os.Stderr, "  - detection score cut triggered")
		}
		os.Exit(1)
	}
}
