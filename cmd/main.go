package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/Joaopdiasventura/rinha-de-backend-2026/internal/http/handlers"
	"github.com/Joaopdiasventura/rinha-de-backend-2026/internal/score"
	"github.com/Joaopdiasventura/rinha-de-backend-2026/internal/vector"
)

func main() {
	shardID := os.Getenv("SHARD_ID")
	if shardID == "" {
		shardID = "0"
	}

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

	log.Println("server running on :8080")
	err = http.ListenAndServe(":8080", mux)

	if err != nil {
		panic(err)
	}
}
