package main

import (
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
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
			for _, value := range ref.Vector {
				err = binary.Write(vecFile, binary.LittleEndian, value)
				if err != nil {
					panic(err)
				}
			}

			label := byte(0)

			if ref.Label == "fraud" {
				label = 1
			}

			_, err = labelFile.Write([]byte{label})
			if err != nil {
				panic(err)
			}
		}

		index++
	}
}