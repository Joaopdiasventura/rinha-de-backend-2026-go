package vector

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
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
			t.Fatalf("neighbor %d distance got %.2f want %.2f", index, got[index].Distance, want)
		}

		if got[index].Label != wantLabels[index] {
			t.Fatalf("neighbor %d label got %d want %d", index, got[index].Label, wantLabels[index])
		}
	}

	if score := FraudScore(got); score != 0.6 {
		t.Fatalf("unexpected merged score: got %.1f want 0.6", score)
	}
}

func TestLoadRejectsCorruptedFiles(t *testing.T) {
	tempDir := t.TempDir()
	vecPath := filepath.Join(tempDir, "references.vec")
	labelPath := filepath.Join(tempDir, "references.labels")

	if err := os.WriteFile(vecPath, []byte{1, 2, 3}, 0o600); err != nil {
		t.Fatalf("write vec file: %v", err)
	}

	if err := os.WriteFile(labelPath, []byte{0}, 0o600); err != nil {
		t.Fatalf("write label file: %v", err)
	}

	if _, err := Load(vecPath, labelPath); err == nil {
		t.Fatal("expected Load to reject misaligned vector file")
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
		t.Fatalf("write vec file: %v", err)
	}

	if err := os.WriteFile(labelPath, []byte{1}, 0o600); err != nil {
		t.Fatalf("write label file: %v", err)
	}

	store, err := Load(vecPath, labelPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if store.Count != 1 {
		t.Fatalf("unexpected count: got %d want 1", store.Count)
	}

	if store.Labels[0] != 1 {
		t.Fatalf("unexpected label: got %d want 1", store.Labels[0])
	}

	if got := store.Search(InputVector{}); got[0].Index != 0 {
		t.Fatalf("unexpected top neighbor index: got %d want 0", got[0].Index)
	}
}
