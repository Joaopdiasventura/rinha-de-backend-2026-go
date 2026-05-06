package vector

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestMergeTopKPreservesGlobalOrdering(t *testing.T) {
	local := [Neighbors]Neighbor{
		{Distance: 0.20, Label: 1, Index: 0},
		{Distance: 0.21, Label: 0, Index: 1},
		{Distance: 0.22, Label: 0, Index: 2},
		{Distance: 0.23, Label: 0, Index: 3},
		{Distance: 0.24, Label: 1, Index: 4},
	}
	remote := [Neighbors]Neighbor{
		{Distance: 0.10, Label: 1, Index: 0},
		{Distance: 0.19, Label: 1, Index: 1},
		{Distance: 0.25, Label: 0, Index: 2},
		{Distance: 0.26, Label: 0, Index: 3},
		{Distance: 0.27, Label: 0, Index: 4},
	}

	got := MergeTopK(local, remote)

	wantDistances := []float64{0.10, 0.19, 0.20, 0.21, 0.22}
	wantLabels := []byte{1, 1, 1, 0, 0}

	for index, want := range wantDistances {
		if got[index].Distance != want {
			t.Fatalf("unexpected neighbor distance at index %d:\n  got:  %.2f\n  want: %.2f", index, got[index].Distance, want)
		}

		if got[index].Label != wantLabels[index] {
			t.Fatalf("unexpected neighbor label at index %d:\n  got:  %d\n  want: %d", index, got[index].Label, wantLabels[index])
		}
	}

	if score := FraudScore(got); score != 0.6 {
		t.Fatalf("unexpected merged fraud_score:\n  got:  %.1f\n  want: %.1f", score, 0.6)
	}
}

func TestLoadRejectsCorruptedFiles(t *testing.T) {
	tempDir := t.TempDir()
	vecPath := filepath.Join(tempDir, "references.vec")
	labelPath := filepath.Join(tempDir, "references.labels")

	if err := os.WriteFile(vecPath, []byte{1, 2, 3}, 0o600); err != nil {
		t.Fatalf("failed to write temporary vector file:\n  error: %v", err)
	}

	if err := os.WriteFile(labelPath, []byte{0}, 0o600); err != nil {
		t.Fatalf("failed to write temporary label file:\n  error: %v", err)
	}

	if _, err := Load(vecPath, labelPath); err == nil {
		t.Fatal("expected Load to reject a misaligned vector file, but it succeeded")
	}
}

func TestLoadReadsAlignedDataset(t *testing.T) {
	tempDir := t.TempDir()
	vecPath := filepath.Join(tempDir, "references.vec")
	labelPath := filepath.Join(tempDir, "references.labels")

	buffer := make([]byte, Dimensions*4)
	for index := range Dimensions {
		binary.LittleEndian.PutUint32(buffer[index*4:index*4+4], math.Float32bits(float32(index)/10))
	}

	if err := os.WriteFile(vecPath, buffer, 0o600); err != nil {
		t.Fatalf("failed to write temporary vector file:\n  error: %v", err)
	}

	if err := os.WriteFile(labelPath, []byte{1}, 0o600); err != nil {
		t.Fatalf("failed to write temporary label file:\n  error: %v", err)
	}

	store, err := Load(vecPath, labelPath)
	if err != nil {
		t.Fatalf("Load returned an unexpected error:\n  error: %v", err)
	}

	if store.Count != 1 {
		t.Fatalf("unexpected vector count:\n  got:  %d\n  want: %d", store.Count, 1)
	}

	if store.Labels[0] != 1 {
		t.Fatalf("unexpected label:\n  got:  %d\n  want: %d", store.Labels[0], 1)
	}

	if got := store.Search(InputVector{}); got[0].Index != 0 {
		t.Fatalf("unexpected top neighbor index:\n  got:  %d\n  want: %d", got[0].Index, 0)
	}
}

func TestSearchConcurrent(t *testing.T) {
	store := benchmarkStore(2048)
	input := benchmarkVector()

	var group sync.WaitGroup
	for i := 0; i < 32; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for j := 0; j < 128; j++ {
				neighbors := store.Search(input)
				if neighbors[0].Index < 0 {
					t.Error("expected a valid nearest neighbor index, but got a negative index")
					return
				}
			}
		}()
	}

	group.Wait()
}
