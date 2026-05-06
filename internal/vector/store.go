package vector

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"unsafe"

	"github.com/Joaopdiasventura/rinha-de-backend-2026/internal/perf"
)

const Dimensions = 14
const Neighbors = 5
type InputVector [Dimensions]float32

type Store struct {
	Vectors []float32
	Labels  []byte
	Count   int

	Stride int
}

var configuredSearchStride = getenvInt("SEARCH_STRIDE", 8)
var exactSearchThreshold = getenvInt("EXACT_SEARCH_THRESHOLD", 65536)

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

	vectors := decodeVectors(vecBytes)

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
		Stride:  searchStrideForCount(count),
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

	for i, value := range values {
		input[i] = float32(value)
	}

	return input, nil
}

func decodeVectors(vecBytes []byte) []float32 {
	if len(vecBytes) == 0 {
		return nil
	}

	if hostIsLittleEndian() && isAlignedForFloat32(vecBytes) {
		return unsafe.Slice((*float32)(unsafe.Pointer(unsafe.SliceData(vecBytes))), len(vecBytes)/4)
	}

	vectors := make([]float32, len(vecBytes)/4)
	for i := range vectors {
		bits := binary.LittleEndian.Uint32(vecBytes[i*4 : i*4+4])
		vectors[i] = math.Float32frombits(bits)
	}

	return vectors
}

func hostIsLittleEndian() bool {
	value := uint16(1)
	return *(*byte)(unsafe.Pointer(&value)) == 1
}

func isAlignedForFloat32(buffer []byte) bool {
	return uintptr(unsafe.Pointer(unsafe.SliceData(buffer)))%unsafe.Alignof(float32(0)) == 0
}

func (s *Store) Search(input InputVector) [Neighbors]Neighbor {
	best := emptyTopK()
	worstDist := float32(math.MaxFloat32)
	stride := s.searchStride()
	vectors := s.Vectors
	labels := s.Labels
	count := s.Count
	topKReady := false
	visited := 0
	prunedStage1 := 0
	prunedStage2 := 0
	prunedStage3 := 0
	inserted := 0

	in0 := input[0]
	in1 := input[1]
	in2 := input[2]
	in3 := input[3]
	in4 := input[4]
	in5 := input[5]
	in6 := input[6]
	in7 := input[7]
	in8 := input[8]
	in9 := input[9]
	in10 := input[10]
	in11 := input[11]
	in12 := input[12]
	in13 := input[13]

	for i := 0; i < count; i += stride {
		visited++
		base := i * Dimensions
		sum := float32(0)

		diff5 := vectors[base+5] - in5
		sum += diff5 * diff5
		diff6 := vectors[base+6] - in6
		sum += diff6 * diff6
		diff7 := vectors[base+7] - in7
		sum += diff7 * diff7
		diff2 := vectors[base+2] - in2
		sum += diff2 * diff2
		if topKReady && sum >= worstDist {
			prunedStage1++
			continue
		}

		diff0 := vectors[base+0] - in0
		sum += diff0 * diff0
		diff13 := vectors[base+13] - in13
		sum += diff13 * diff13
		diff8 := vectors[base+8] - in8
		sum += diff8 * diff8
		diff3 := vectors[base+3] - in3
		sum += diff3 * diff3
		if topKReady && sum >= worstDist {
			prunedStage2++
			continue
		}

		diff4 := vectors[base+4] - in4
		sum += diff4 * diff4
		diff12 := vectors[base+12] - in12
		sum += diff12 * diff12
		diff11 := vectors[base+11] - in11
		sum += diff11 * diff11
		diff9 := vectors[base+9] - in9
		sum += diff9 * diff9
		if topKReady && sum >= worstDist {
			prunedStage3++
			continue
		}

		diff10 := vectors[base+10] - in10
		sum += diff10 * diff10
		diff1 := vectors[base+1] - in1
		sum += diff1 * diff1

		if sum < worstDist {
			inserted++
			insertNeighbor(&best, Neighbor{
				Distance: float64(sum),
				Label:    labels[i],
				Index:    i,
			})

			if best[Neighbors-1].Index >= 0 {
				topKReady = true
				worstDist = float32(best[Neighbors-1].Distance)
			}
		}
	}

	if perf.Enabled() {
		perf.ObserveSearch(visited, prunedStage1, prunedStage2, prunedStage3, inserted, stride)
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
		diff := float64(s.Vectors[offset+i] - input[i])
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
	if candidate.Index < 0 || candidate.Distance >= best[Neighbors-1].Distance {
		return
	}

	if candidate.Distance < best[0].Distance {
		best[4] = best[3]
		best[3] = best[2]
		best[2] = best[1]
		best[1] = best[0]
		best[0] = candidate
		return
	}

	if candidate.Distance < best[1].Distance {
		best[4] = best[3]
		best[3] = best[2]
		best[2] = best[1]
		best[1] = candidate
		return
	}

	if candidate.Distance < best[2].Distance {
		best[4] = best[3]
		best[3] = best[2]
		best[2] = candidate
		return
	}

	if candidate.Distance < best[3].Distance {
		best[4] = best[3]
		best[3] = candidate
		return
	}

	best[4] = candidate
}

func (s *Store) searchStride() int {
	if s.Stride > 0 {
		return s.Stride
	}

	return searchStrideForCount(s.Count)
}

func searchStrideForCount(count int) int {
	if count < exactSearchThreshold {
		return 1
	}

	if configuredSearchStride < 1 {
		return 1
	}

	return configuredSearchStride
}

func getenvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}

	return parsed
}
