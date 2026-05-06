package score

import (
	"context"
	"errors"
	"time"

	"github.com/Joaopdiasventura/rinha-de-backend-2026/internal/constants"
	"github.com/Joaopdiasventura/rinha-de-backend-2026/internal/http/dto"
	"github.com/Joaopdiasventura/rinha-de-backend-2026/internal/perf"
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
	result, _, err := s.validateScore(ctx, transaction)
	return result, err
}

type Timings struct {
	Vectorize time.Duration
	Search    time.Duration
}

func (s *Service) ValidateScoreWithTimings(ctx context.Context, transaction dto.ValidateTransactionDTO) (dto.Score, Timings, error) {
	return s.validateScore(ctx, transaction)
}

func (s *Service) validateScore(ctx context.Context, transaction dto.ValidateTransactionDTO) (dto.Score, Timings, error) {
	_ = ctx

	timings := Timings{}
	vectorizeStart := time.Time{}
	if perf.Enabled() {
		vectorizeStart = time.Now()
	}

	input := buildVector(transaction)

	if perf.Enabled() {
		timings.Vectorize = time.Since(vectorizeStart)
	}

	searchStart := time.Time{}
	if perf.Enabled() {
		searchStart = time.Now()
	}
	neighbors := s.Store.Search(input)
	if perf.Enabled() {
		timings.Search = time.Since(searchStart)
	}
	fraudScore := vector.FraudScore(neighbors)

	return dto.Score{
		Approved:   fraudScore < 0.6,
		FraudScore: fraudScore,
	}, timings, nil
}

func buildVector(transaction dto.ValidateTransactionDTO) vector.InputVector {
	output := vector.InputVector{}
	requestedAtUTC := transaction.Transaction.RequestedAt.UTC()

	output[0] = normalize(transaction.Transaction.Amount / constants.MaxAmount)
	output[1] = normalize(float64(transaction.Transaction.Installments) / constants.MaxInstallments)
	output[2] = normalize((transaction.Transaction.Amount / transaction.Customer.AvgAmount) / constants.AmountVsAvgRatio)
	output[3] = normalize(float64(requestedAtUTC.Hour()) / 23)
	output[4] = normalizeWeekday(requestedAtUTC)
	output[5] = -1
	output[6] = -1

	if transaction.LastTransaction != nil {
		minutes := requestedAtUTC.Sub(transaction.LastTransaction.Timestamp).Minutes()
		output[5] = clamp(minutes / constants.MaxMinutes)
		output[6] = clamp(transaction.LastTransaction.KmFromCurrent / constants.MaxKm)
	}

	output[7] = clamp(transaction.Terminal.KmFromHome / constants.MaxKm)
	output[8] = clamp(float64(transaction.Customer.TxCount24h) / constants.MaxTxCount24h)
	output[9] = boolToFloat(transaction.Terminal.IsOnline)
	output[10] = boolToFloat(transaction.Terminal.CardPresent)
	output[11] = boolToFloat(isUnknownMerchant(transaction.Customer.KnownMerchants, transaction.Merchant.ID))
	output[12] = mccRisk(transaction.Merchant.MCC)
	output[13] = clamp(transaction.Merchant.AvgAmount / constants.MaxMerchantAvgAmount)

	return output
}

func isUnknownMerchant(knownMerchants []string, merchantID string) bool {
	for _, knownMerchant := range knownMerchants {
		if knownMerchant == merchantID {
			return false
		}
	}

	return true
}

func clamp(value float64) float32 {
	if value < 0 {
		return 0
	}

	if value > 1 {
		return 1
	}

	return round(float32(value))
}

func normalize(value float64) float32 {
	return round(clamp(value))
}

func round(value float32) float32 {
	return float32(int(value*10000+0.5)) / 10000
}

func boolToFloat(value bool) float32 {
	if value {
		return 1
	}

	return 0
}

func normalizeWeekday(value time.Time) float32 {
	weekday := value.UTC().Weekday()

	if weekday == time.Sunday {
		return 1
	}

	return round(float32(int(weekday)-1) / 6)
}

func mccRisk(mcc string) float32 {
	value, exists := constants.MccRisk[mcc]

	if !exists {
		return 0.5
	}

	return float32(value)
}
