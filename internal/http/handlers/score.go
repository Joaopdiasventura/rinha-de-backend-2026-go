package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Joaopdiasventura/rinha-de-backend-2026/internal/http/dto"
	"github.com/Joaopdiasventura/rinha-de-backend-2026/internal/perf"
	"github.com/Joaopdiasventura/rinha-de-backend-2026/internal/score"
)

func FraudScoreHandler(scoreService *score.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		totalStart := time.Time{}
		decodeStart := time.Time{}
		if perf.Enabled() {
			totalStart = time.Now()
			decodeStart = totalStart
		}

		var body dto.ValidateTransactionDTO

		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil {
			perf.ObserveDecodeError()
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}

		decodeDuration := time.Duration(0)
		if perf.Enabled() {
			decodeDuration = time.Since(decodeStart)
		}

		result, timings, err := scoreService.ValidateScoreWithTimings(r.Context(), body)
		if err != nil {
			perf.ObserveServiceError()
			http.Error(w, "peer unavailable", http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		encodeStart := time.Time{}
		if perf.Enabled() {
			encodeStart = time.Now()
		}

		err = json.NewEncoder(w).Encode(result)
		if err != nil {
			perf.ObserveEncodeError()
			http.Error(w, "failed to encode response", http.StatusInternalServerError)
			return
		}

		if perf.Enabled() {
			encodeDuration := time.Since(encodeStart)
			perf.ObserveRequest(
				decodeDuration,
				timings.Vectorize,
				timings.Search,
				encodeDuration,
				time.Since(totalStart),
			)
		}
	}
}
