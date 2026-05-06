package score

import (
	"context"
	"testing"
)

func BenchmarkBuildVector(b *testing.B) {
	tx := validTransaction()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = buildVector(tx)
	}
}

func BenchmarkValidateScore(b *testing.B) {
	tx := validTransaction()
	base := buildVector(tx)
	store := buildStoreAround(base, []neighborSpec{
		{Distance: 0.11, Label: 1},
		{Distance: 0.12, Label: 0},
		{Distance: 0.13, Label: 0},
		{Distance: 0.14, Label: 1},
		{Distance: 0.15, Label: 0},
		{Distance: 0.16, Label: 0},
		{Distance: 0.17, Label: 1},
		{Distance: 0.18, Label: 0},
		{Distance: 0.19, Label: 0},
		{Distance: 0.20, Label: 1},
	})
	service := NewService(store)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := service.ValidateScore(context.Background(), tx); err != nil {
			b.Fatalf("ValidateScore returned error: %v", err)
		}
	}
}
