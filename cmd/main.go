package main

import (
	"log"
	"net/http"

	"github.com/Joaopdiasventura/rinha-de-backend-2026/internal/http/handlers"
	"github.com/Joaopdiasventura/rinha-de-backend-2026/internal/score"
	"github.com/Joaopdiasventura/rinha-de-backend-2026/internal/vector"
)

func main() {
	store, err := vector.Load("/app/resources/references.vec", "/app/resources/references.labels")
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
