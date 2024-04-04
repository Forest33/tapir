package encryptor

import (
	"os"
	"strings"
	"testing"
)

var (
	testDataPath = "README.md"
	key          = strings.Repeat("0", 32)
)

func getTestData() []byte {
	data, err := os.ReadFile(testDataPath)
	if err != nil {
		panic(err)
	}
	return data
}

func BenchmarkAESECB_Encrypt(b *testing.B) {
	enc := NewAESECB(key)
	data := getTestData()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		out, _ := enc.Encrypt(data)
		l := enc.GetLength(len(data))
		_ = out
		_ = l
	}
}

func BenchmarkAESECB_Decrypt(b *testing.B) {
	enc := NewAESECB(key)
	data, _ := enc.Encrypt(getTestData())
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		out, _ := enc.Decrypt(data)
		_ = out
	}
}

func BenchmarkAESGCM_Encrypt(b *testing.B) {
	enc := NewAESGCM(key)
	data := getTestData()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		out, _ := enc.Encrypt(data)
		l := enc.GetLength(len(data))
		_ = out
		_ = l
	}
}

func BenchmarkAESGCM_Decrypt(b *testing.B) {
	enc := NewAESGCM(key)
	data, _ := enc.Encrypt(getTestData())
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		out, _ := enc.Decrypt(data)
		_ = out
	}
}
