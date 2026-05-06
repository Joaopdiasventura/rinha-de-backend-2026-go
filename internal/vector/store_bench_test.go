package vector

import "testing"

func BenchmarkSearch(b *testing.B) {
	store := benchmarkStore(32768)
	input := benchmarkVector()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = store.Search(input)
	}
}

func BenchmarkFromSlice(b *testing.B) {
	values := make([]float64, Dimensions)
	for i := range Dimensions {
		values[i] = float64(i) / 10
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := FromSlice(values)
		if err != nil {
			b.Fatalf("FromSlice returned error: %v", err)
		}
	}
}

func BenchmarkMergeTopK(b *testing.B) {
	local := [Neighbors]Neighbor{
		{Distance: 0.11, Label: 1, Index: 0},
		{Distance: 0.21, Label: 0, Index: 1},
		{Distance: 0.31, Label: 0, Index: 2},
		{Distance: 0.41, Label: 0, Index: 3},
		{Distance: 0.51, Label: 1, Index: 4},
	}
	remote := [Neighbors]Neighbor{
		{Distance: 0.12, Label: 0, Index: 10},
		{Distance: 0.22, Label: 1, Index: 11},
		{Distance: 0.32, Label: 0, Index: 12},
		{Distance: 0.42, Label: 1, Index: 13},
		{Distance: 0.52, Label: 0, Index: 14},
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = MergeTopK(local, remote)
	}
}

func benchmarkStore(count int) *Store {
	vectors := make([]float32, 0, count*Dimensions)
	labels := make([]byte, 0, count)

	for i := 0; i < count; i++ {
		for d := 0; d < Dimensions; d++ {
			vectors = append(vectors, float32((i+d)%1000)/1000)
		}

		labels = append(labels, byte(i%2))
	}

	return &Store{
		Vectors: vectors,
		Labels:  labels,
		Count:   count,
	}
}

func benchmarkVector() InputVector {
	var input InputVector

	for i := range input {
		input[i] = float32(i+1) / 100
	}

	return input
}
