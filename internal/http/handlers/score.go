package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/Joaopdiasventura/rinha-de-backend-2026/internal/http/dto"
	"github.com/Joaopdiasventura/rinha-de-backend-2026/internal/score"
)

func FraudScoreHandler(scoreService *score.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body dto.ValidateTransactionDTO

		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}

		result, err := scoreService.ValidateScore(r.Context(), body)
		if err != nil {
			http.Error(w, "peer unavailable", http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		err = json.NewEncoder(w).Encode(result)
		if err != nil {
			http.Error(w, "failed to encode response", http.StatusInternalServerError)
			return
		}
	}
}
