package compression

import (
	"os"
	"testing"
)

const (
	testDataPath         = "README.md"
	zstdCompressionLevel = 2
)

func getTestData() []byte {
	data, err := os.ReadFile(testDataPath)
	if err != nil {
		panic(err)
	}
	return data
}

func BenchmarkCompressLZ4(b *testing.B) {
	data := getTestData()
	cmp := New(&Config{PayloadSize: len(data)})
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		out, _ := cmp.CompressLZ4(data)
		_ = out
	}
}

func BenchmarkDecompressLZ4(b *testing.B) {
	cmp := New(&Config{PayloadSize: len(getTestData())})
	data, _ := cmp.CompressLZ4(getTestData())
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		out, _ := cmp.DecompressLZ4(data)
		_ = out
	}
}

func BenchmarkCompressLZO(b *testing.B) {
	data := getTestData()
	cmp := New(&Config{PayloadSize: len(data)})
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		out, _ := cmp.CompressLZO(data)
		_ = out
	}
}

func BenchmarkDecompressLZO(b *testing.B) {
	cmp := New(&Config{PayloadSize: len(getTestData())})
	data, _ := cmp.CompressLZO(getTestData())
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		out, _ := cmp.DecompressLZO(data)
		_ = out
	}
}

func BenchmarkCompressZSTD(b *testing.B) {
	data := getTestData()
	cmp := New(&Config{PayloadSize: len(data)})
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		out, _ := cmp.CompressZSTD(data, zstdCompressionLevel)
		_ = out
	}
}

func BenchmarkDecompressZSTD(b *testing.B) {
	cmp := New(&Config{PayloadSize: len(getTestData())})
	data, _ := cmp.CompressZSTD(getTestData(), zstdCompressionLevel)
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		out, _ := cmp.DecompressZSTD(data)
		_ = out
	}
}
