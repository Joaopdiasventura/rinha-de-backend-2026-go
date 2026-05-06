package score

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Joaopdiasventura/rinha-de-backend-2026/internal/http/dto"
	"github.com/Joaopdiasventura/rinha-de-backend-2026/internal/vector"
)

func TestBuildVectorExamples(t *testing.T) {
	t.Run("legit example", func(t *testing.T) {
		tx := dto.ValidateTransactionDTO{
			ID: "tx-1329056812",
			Transaction: dto.TransactionDTO{
				Amount:       41.12,
				Installments: 2,
				RequestedAt:  time.Date(2026, 3, 11, 18, 45, 53, 0, time.UTC),
			},
			Customer: dto.CustomerDTO{
				AvgAmount:      82.24,
				TxCount24h:     3,
				KnownMerchants: []string{"MERC-003", "MERC-016"},
			},
			Merchant: dto.MerchantDTO{
				ID:        "MERC-016",
				MCC:       "5411",
				AvgAmount: 60.25,
			},
			Terminal: dto.TerminalDTO{
				IsOnline:    false,
				CardPresent: true,
				KmFromHome:  29.2331036248,
			},
			LastTransaction: nil,
		}

		got := buildVector(tx)
		want := vector.InputVector{0.0041, 0.1667, 0.05, 0.7826, 0.3333, -1, -1, 0.0292, 0.15, 0, 1, 0, 0.15, 0.006}

		if got != want {
			t.Fatalf("unexpected vector:\n  got:  %v\n  want: %v", got, want)
		}
	})

	t.Run("fraud example", func(t *testing.T) {
		tx := dto.ValidateTransactionDTO{
			ID: "tx-3330991687",
			Transaction: dto.TransactionDTO{
				Amount:       9505.97,
				Installments: 10,
				RequestedAt:  time.Date(2026, 3, 14, 5, 15, 12, 0, time.UTC),
			},
			Customer: dto.CustomerDTO{
				AvgAmount:      81.28,
				TxCount24h:     20,
				KnownMerchants: []string{"MERC-008", "MERC-007", "MERC-005"},
			},
			Merchant: dto.MerchantDTO{
				ID:        "MERC-068",
				MCC:       "7802",
				AvgAmount: 54.86,
			},
			Terminal: dto.TerminalDTO{
				IsOnline:    false,
				CardPresent: true,
				KmFromHome:  952.27,
			},
			LastTransaction: nil,
		}

		got := buildVector(tx)
		want := vector.InputVector{0.9506, 0.8333, 1, 0.2174, 0.8333, -1, -1, 0.9523, 1, 0, 1, 1, 0.75, 0.0055}

		if got != want {
			t.Fatalf("unexpected vector:\n  got:  %v\n  want: %v", got, want)
		}
	})
}

func TestValidateScoreApprovesWhenFraudScoreBelowThreshold(t *testing.T) {
	tx := validTransaction()
	base := buildVector(tx)

	store := buildStoreAround(base, []neighborSpec{
		{Distance: 0.20, Label: 1},
		{Distance: 0.21, Label: 0},
		{Distance: 0.22, Label: 0},
		{Distance: 0.23, Label: 0},
		{Distance: 0.24, Label: 1},
	})

	service := NewService(store)

	got, err := service.ValidateScore(context.Background(), tx)
	if err != nil {
		t.Fatalf("ValidateScore returned an unexpected error:\n  error: %v", err)
	}

	if got.FraudScore != 0.4 {
		t.Fatalf("unexpected fraud_score:\n  got:  %.1f\n  want: %.1f", got.FraudScore, 0.4)
	}

	if !got.Approved {
		t.Fatal("expected transaction to be approved, but it was rejected")
	}
}

func TestValidateScoreRejectsWhenFraudScoreReachesThreshold(t *testing.T) {
	tx := validTransaction()
	base := buildVector(tx)

	store := buildStoreAround(base, []neighborSpec{
		{Distance: 0.20, Label: 1},
		{Distance: 0.21, Label: 1},
		{Distance: 0.22, Label: 1},
		{Distance: 0.23, Label: 0},
		{Distance: 0.24, Label: 0},
	})

	service := NewService(store)

	got, err := service.ValidateScore(context.Background(), tx)
	if err != nil {
		t.Fatalf("ValidateScore returned an unexpected error:\n  error: %v", err)
	}

	if got.FraudScore != 0.6 {
		t.Fatalf("unexpected fraud_score:\n  got:  %.1f\n  want: %.1f", got.FraudScore, 0.6)
	}

	if got.Approved {
		t.Fatal("expected transaction to be rejected, but it was approved")
	}
}

func TestValidateScoreConcurrent(t *testing.T) {
	tx := validTransaction()
	base := buildVector(tx)
	store := buildStoreAround(base, []neighborSpec{
		{Distance: 0.20, Label: 1},
		{Distance: 0.21, Label: 1},
		{Distance: 0.22, Label: 1},
		{Distance: 0.23, Label: 0},
		{Distance: 0.24, Label: 0},
		{Distance: 0.25, Label: 0},
		{Distance: 0.26, Label: 0},
	})
	service := NewService(store)

	var group sync.WaitGroup
	for i := 0; i < 32; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for j := 0; j < 128; j++ {
				got, err := service.ValidateScore(context.Background(), tx)
				if err != nil {
					t.Errorf("ValidateScore returned an unexpected error:\n  error: %v", err)
					return
				}

				if got.FraudScore != 0.6 {
					t.Errorf("unexpected fraud_score:\n  got:  %.1f\n  want: %.1f", got.FraudScore, 0.6)
					return
				}
			}
		}()
	}

	group.Wait()
}

func validTransaction() dto.ValidateTransactionDTO {
	return dto.ValidateTransactionDTO{
		ID: "tx-1329056812",
		Transaction: dto.TransactionDTO{
			Amount:       41.12,
			Installments: 2,
			RequestedAt:  time.Date(2026, 3, 11, 18, 45, 53, 0, time.UTC),
		},
		Customer: dto.CustomerDTO{
			AvgAmount:      82.24,
			TxCount24h:     3,
			KnownMerchants: []string{"MERC-003", "MERC-016"},
		},
		Merchant: dto.MerchantDTO{
			ID:        "MERC-016",
			MCC:       "5411",
			AvgAmount: 60.25,
		},
		Terminal: dto.TerminalDTO{
			IsOnline:    false,
			CardPresent: true,
			KmFromHome:  29.2331036248,
		},
		LastTransaction: nil,
	}
}

type neighborSpec struct {
	Distance float64
	Label    byte
}

func buildStoreAround(base vector.InputVector, specs []neighborSpec) *vector.Store {
	values := make([]float32, 0, len(specs)*vector.Dimensions)
	labels := make([]byte, 0, len(specs))

	for _, spec := range specs {
		current := base
		current[0] += float32(spec.Distance)

		for _, value := range current {
			values = append(values, float32(value))
		}

		labels = append(labels, spec.Label)
	}

	return &vector.Store{
		Vectors: values,
		Labels:  labels,
		Count:   len(labels),
	}
}
