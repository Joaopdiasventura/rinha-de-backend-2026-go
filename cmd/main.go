package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/Joaopdiasventura/rinha-de-backend-2026/internal/diagnostics"
	"github.com/Joaopdiasventura/rinha-de-backend-2026/internal/http/handlers"
	"github.com/Joaopdiasventura/rinha-de-backend-2026/internal/perf"
	"github.com/Joaopdiasventura/rinha-de-backend-2026/internal/score"
	"github.com/Joaopdiasventura/rinha-de-backend-2026/internal/vector"
)

func main() {
	runtime.GOMAXPROCS(getenvInt("GOMAXPROCS", 1))
	shardID := getenv("SHARD_ID", "0")
	diagnosticsEnabled := getenvBool("DIAGNOSTICS_ENABLED", false)
	perf.Configure(diagnosticsEnabled)

	vecPath := fmt.Sprintf("/app/resources/%s/references.vec", shardID)
	labelPath := fmt.Sprintf("/app/resources/%s/references.labels", shardID)

	store, err := vector.Load(vecPath, labelPath)
	if err != nil {
		panic(err)
	}

	scoreService := score.NewService(store)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ready", handlers.ReadyHandler())
	mux.HandleFunc("POST /fraud-score", handlers.FraudScoreHandler(scoreService))

	if diagnosticsEnabled {
		go runDiagnosticsServer(getenv("DIAGNOSTICS_PORT", "6060"))
	}

	server := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	log.Println("server running on :8080")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}

func runDiagnosticsServer(port string) {
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           diagnostics.NewHandler(),
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	log.Printf("diagnostics server running on :%s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("diagnostics server stopped: %v", err)
	}
}

func getenv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}

func getenvBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func getenvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}
