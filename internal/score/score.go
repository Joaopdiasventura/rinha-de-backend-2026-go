package score

import (
	"context"
	"errors"
	"math"
	"slices"
	"time"

	"github.com/Joaopdiasventura/rinha-de-backend-2026/internal/constants"
	"github.com/Joaopdiasventura/rinha-de-backend-2026/internal/http/dto"
	"github.com/Joaopdiasventura/rinha-de-backend-2026/internal/vector"
)

var ErrPeerUnavailable = errors.New("peer unavailable")

type Service struct {
	Store *vector.Store
}

func NewService(store *vector.Store) *Service {
	return &Service{
		Store: store,
	}
}

func (s *Service) ValidateScore(ctx context.Context, transaction dto.ValidateTransactionDTO) (dto.Score, error) {

	input := buildVector(transaction)

	results := make(chan [vector.Neighbors]vector.Neighbor, 1)

	go func() {
		results <- s.Store.Search(input)
	}()

	result := <-results

	fraudScore := vector.FraudScore(result)

	return dto.Score{
		Approved:   fraudScore < 0.6,
		FraudScore: fraudScore,
	}, nil
}

func buildVector(transaction dto.ValidateTransactionDTO) vector.InputVector {
	output := vector.InputVector{}

	output[0] = normalize(transaction.Transaction.Amount / constants.MaxAmount)
	output[1] = normalize(float64(transaction.Transaction.Installments) / constants.MaxInstallments)
	output[2] = normalize((transaction.Transaction.Amount / transaction.Customer.AvgAmount) / constants.AmountVsAvgRatio)
	output[3] = normalize(float64(transaction.Transaction.RequestedAt.UTC().Hour()) / 23)
	output[4] = normalizeWeekday(transaction.Transaction.RequestedAt)
	output[5] = -1
	output[6] = -1

	if transaction.LastTransaction != nil {
		minutes := transaction.Transaction.RequestedAt.Sub(transaction.LastTransaction.Timestamp).Minutes()
		output[5] = clamp(minutes / constants.MaxMinutes)
		output[6] = clamp(transaction.LastTransaction.KmFromCurrent / constants.MaxKm)
	}

	output[7] = clamp(transaction.Terminal.KmFromHome / constants.MaxKm)
	output[8] = clamp(float64(transaction.Customer.TxCount24h) / constants.MaxTxCount24h)
	output[9] = boolToFloat(transaction.Terminal.IsOnline)
	output[10] = boolToFloat(transaction.Terminal.CardPresent)
	output[11] = boolToFloat(!slices.Contains(transaction.Customer.KnownMerchants, transaction.Merchant.ID))
	output[12] = mccRisk(transaction.Merchant.MCC)
	output[13] = clamp(transaction.Merchant.AvgAmount / constants.MaxMerchantAvgAmount)

	return output
}

func clamp(value float64) float64 {
	if value < 0 {
		return 0
	}

	if value > 1 {
		return 1
	}

	return round(value, 4)
}

func normalize(value float64) float64 {
	return round(clamp(value), 4)
}

func round(value float64, precision int) float64 {
	pow := math.Pow(10, float64(precision))
	return math.Round(value*pow) / pow
}

func boolToFloat(value bool) float64 {
	if value {
		return 1
	}

	return 0
}

func normalizeWeekday(value time.Time) float64 {
	weekday := value.UTC().Weekday()

	if weekday == time.Sunday {
		return 1
	}

	return round(float64(int(weekday)-1)/6, 4)
}

func mccRisk(mcc string) float64 {
	value, exists := constants.MccRisk[mcc]

	if !exists {
		return 0.5
	}

	return value
}
