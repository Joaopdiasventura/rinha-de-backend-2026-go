package vector

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"os"
)

const Dimensions = 14
const Neighbors = 5
const SearchStride = 1

type InputVector [Dimensions]float64

type Store struct {
	Vectors []float32
	Labels  []byte
	Count   int
}

type Neighbor struct {
	Distance float64
	Label    byte
	Index    int
}

func Load(vecPath string, labelsPath string) (*Store, error) {
	vecBytes, err := os.ReadFile(vecPath)
	if err != nil {
		return nil, err
	}

	labels, err := os.ReadFile(labelsPath)
	if err != nil {
		return nil, err
	}

	if len(vecBytes)%4 != 0 {
		return nil, fmt.Errorf("vector file size is not aligned to float32 values: %d", len(vecBytes))
	}

	if len(vecBytes)%(Dimensions*4) != 0 {
		return nil, fmt.Errorf("vector file size is not aligned to %d dimensions: %d", Dimensions, len(vecBytes))
	}

	count := len(vecBytes) / (Dimensions * 4)
	if len(labels) != count {
		return nil, fmt.Errorf("labels count (%d) does not match vector count (%d)", len(labels), count)
	}

	vectors := make([]float32, len(vecBytes)/4)
	for i := range vectors {
		bits := binary.LittleEndian.Uint32(vecBytes[i*4 : i*4+4])
		vectors[i] = math.Float32frombits(bits)
	}

	frauds := 0
	for index, label := range labels {
		switch label {
		case 0:
		case 1:
			frauds++
		default:
			return nil, fmt.Errorf("invalid label %d at index %d", label, index)
		}
	}

	store := &Store{
		Vectors: vectors,
		Labels:  labels,
		Count:   count,
	}

	log.Printf(
		"loaded shard vec_bytes=%d label_bytes=%d count=%d legit=%d fraud=%d",
		len(vecBytes),
		len(labels),
		count,
		count-frauds,
		frauds,
	)

	return store, nil
}

func FromSlice(values []float64) (InputVector, error) {
	var input InputVector

	if len(values) != Dimensions {
		return input, fmt.Errorf("invalid vector dimension: got %d want %d", len(values), Dimensions)
	}

	copy(input[:], values)
	return input, nil
}

func (s *Store) Search(input InputVector) [Neighbors]Neighbor {
	best := emptyTopK()
	worstDist := math.MaxFloat64

	for i := 0; i < s.Count; i += SearchStride {
		dist := s.squaredDistance(i, input)

		if dist < worstDist {
			insertNeighbor(&best, Neighbor{
				Distance: dist,
				Label:    s.Labels[i],
				Index:    i,
			})
			worstDist = best[Neighbors-1].Distance
		}
	}

	return best
}

func MergeTopK(groups ...[Neighbors]Neighbor) [Neighbors]Neighbor {
	best := emptyTopK()

	for _, group := range groups {
		for _, candidate := range group {
			insertNeighbor(&best, candidate)
		}
	}

	return best
}

func FraudScore(neighbors [Neighbors]Neighbor) float64 {
	frauds := 0

	for _, neighbor := range neighbors {
		if neighbor.Label == 1 {
			frauds++
		}
	}

	return float64(frauds) / Neighbors
}

func (s *Store) squaredDistance(index int, input InputVector) float64 {
	offset := index * Dimensions
	sum := 0.0

	for i := range Dimensions {
		diff := float64(s.Vectors[offset+i]) - input[i]
		sum += diff * diff
	}

	return sum
}

func emptyTopK() [Neighbors]Neighbor {
	best := [Neighbors]Neighbor{}

	for i := range best {
		best[i] = Neighbor{
			Distance: math.MaxFloat64,
			Index:    -1,
		}
	}

	return best
}

func insertNeighbor(best *[Neighbors]Neighbor, candidate Neighbor) {
	if candidate.Index < 0 {
		return
	}

	for i := range Neighbors {
		if candidate.Distance < best[i].Distance {
			copy(best[i+1:], best[i:Neighbors-1])
			best[i] = candidate
			return
		}
	}
}
