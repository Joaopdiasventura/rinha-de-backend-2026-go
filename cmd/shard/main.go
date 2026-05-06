package main

import (
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Joaopdiasventura/rinha-de-backend-2026/internal/vector"
)

type Reference struct {
	Vector []float32 `json:"vector"`
	Label  string    `json:"label"`
}

func main() {
	inputPath := flag.String("input", "resources/references.json.gz", "")
	outDir := flag.String("out", "resources", "")
	shardID := flag.Int("shard-id", 0, "")
	shardCount := flag.Int("shard-count", 2, "")

	flag.Parse()

	input, err := os.Open(*inputPath)
	if err != nil {
		panic(err)
	}
	defer input.Close()

	var reader io.Reader = input

	if strings.HasSuffix(*inputPath, ".gz") {
		gz, err := gzip.NewReader(input)
		if err != nil {
			panic(err)
		}
		defer gz.Close()

		reader = gz
	}

	err = os.MkdirAll(*outDir, 0755)
	if err != nil {
		panic(err)
	}

	vecPath := filepath.Join(*outDir, "references.vec")
	labelPath := filepath.Join(*outDir, "references.labels")

	vecFile, err := os.Create(vecPath)
	if err != nil {
		panic(err)
	}
	defer vecFile.Close()

	labelFile, err := os.Create(labelPath)
	if err != nil {
		panic(err)
	}
	defer labelFile.Close()

	decoder := json.NewDecoder(reader)

	_, err = decoder.Token()
	if err != nil {
		panic(err)
	}

	index := 0

	for decoder.More() {
		var ref Reference

		err := decoder.Decode(&ref)
		if err != nil {
			panic(err)
		}

		if index%*shardCount == *shardID {
			if len(ref.Vector) != vector.Dimensions {
				panic(fmt.Errorf("invalid vector dimension at index %d: got %d want %d", index, len(ref.Vector), vector.Dimensions))
			}

			for _, value := range ref.Vector {
				err = binary.Write(vecFile, binary.LittleEndian, value)
				if err != nil {
					panic(err)
				}
			}

			var label byte
			switch ref.Label {
			case "legit":
				label = 0
			case "fraud":
				label = 1
			default:
				panic(fmt.Errorf("invalid label at index %d: %q", index, ref.Label))
			}

			_, err = labelFile.Write([]byte{label})
			if err != nil {
				panic(err)
			}
		}

		index++
	}
}
