package storage

import (
	"bytes"
	"testing"

	"github.com/moritz/rpk/internal/storage"
)

// TestNewCompressor tests compressor creation
func TestNewCompressor(t *testing.T) {
	comp := storage.NewCompressor()
	if comp == nil {
		t.Error("NewCompressor returned nil")
	}
}

// TestCompress tests data compression
func TestCompress(t *testing.T) {
	comp := storage.NewCompressor()
	originalData := []byte("Hello, World! This is test data for compression. " +
		"Lorem ipsum dolor sit amet, consectetur adipiscing elit. " +
		"The quick brown fox jumps over the lazy dog.")

	compressed, err := comp.Compress(originalData)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	if len(compressed) == 0 {
		t.Error("Compressed data is empty")
	}

	// Compressed data should be smaller for repetitive data
	if len(compressed) >= len(originalData) {
		t.Logf("Warning: Compressed data (%d bytes) is not smaller than original (%d bytes)",
			len(compressed), len(originalData))
	}
}

// TestCompressEmpty tests compressing empty data
func TestCompressEmpty(t *testing.T) {
	comp := storage.NewCompressor()
	compressed, err := comp.Compress([]byte{})
	if err != nil {
		t.Fatalf("Compress empty failed: %v", err)
	}

	if len(compressed) != 0 {
		t.Errorf("Expected empty compressed data, got %d bytes", len(compressed))
	}
}

// TestDecompress tests data decompression
func TestDecompress(t *testing.T) {
	comp := storage.NewCompressor()
	originalData := []byte("Test data for compression and decompression. " +
		"This should decompress back to the original data.")

	// Compress
	compressed, err := comp.Compress(originalData)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	// Decompress
	decompressed, err := comp.Decompress(compressed)
	if err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}

	// Verify
	if !bytes.Equal(decompressed, originalData) {
		t.Errorf("Decompressed data doesn't match original.\nExpected: %s\nGot: %s",
			string(originalData), string(decompressed))
	}
}

// TestDecompressEmpty tests decompressing empty data
func TestDecompressEmpty(t *testing.T) {
	comp := storage.NewCompressor()
	decompressed, err := comp.Decompress([]byte{})
	if err != nil {
		t.Fatalf("Decompress empty failed: %v", err)
	}

	if len(decompressed) != 0 {
		t.Errorf("Expected empty decompressed data, got %d bytes", len(decompressed))
	}
}

// TestRoundTrip tests compress then decompress
func TestRoundTrip(t *testing.T) {
	comp := storage.NewCompressor()

	testCases := []struct {
		name string
		data string
	}{
		{"simple text", "Hello, World!"},
		{"repeated text", "aaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{"JSON", `{"key":"value","number":42,"array":[1,2,3]}`},
		{"large text", string(bytes.Repeat([]byte("The quick brown fox jumps over the lazy dog. "), 100))},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			originalData := []byte(tc.data)

			// Compress
			compressed, err := comp.Compress(originalData)
			if err != nil {
				t.Fatalf("Compress failed: %v", err)
			}

			// Decompress
			decompressed, err := comp.Decompress(compressed)
			if err != nil {
				t.Fatalf("Decompress failed: %v", err)
			}

			// Verify
			if !bytes.Equal(decompressed, originalData) {
				t.Errorf("Round trip failed for %s", tc.name)
			}
		})
	}
}

// TestGetCompressionRatio tests compression ratio calculation
func TestGetCompressionRatio(t *testing.T) {
	comp := storage.NewCompressor()
	original := []byte("test data for compression")
	compressed := []byte("compressed")

	ratio := comp.GetCompressionRatio(original, compressed)

	if ratio < 0 || ratio > 1 {
		t.Errorf("Invalid compression ratio: %f (should be between 0 and 1)", ratio)
	}

	expectedRatio := float64(len(compressed)) / float64(len(original))
	if ratio != expectedRatio {
		t.Errorf("Wrong ratio. Expected %f, got %f", expectedRatio, ratio)
	}
}

// TestGetCompressionRatioEmpty tests compression ratio with empty data
func TestGetCompressionRatioEmpty(t *testing.T) {
	comp := storage.NewCompressor()
	ratio := comp.GetCompressionRatio([]byte{}, []byte{})

	if ratio != 0.0 {
		t.Errorf("Expected ratio 0.0 for empty data, got %f", ratio)
	}
}

// TestGetCompressionSavings tests compression savings calculation
func TestGetCompressionSavings(t *testing.T) {
	comp := storage.NewCompressor()
	original := []byte("original data with 50 bytes")
	compressed := []byte("smaller")

	savings := comp.GetCompressionSavings(original, compressed)

	expected := int64(len(original) - len(compressed))
	if savings != expected {
		t.Errorf("Expected savings %d, got %d", expected, savings)
	}
}

// TestIsCompressionEffective tests compression effectiveness check
func TestIsCompressionEffective(t *testing.T) {
	comp := storage.NewCompressor()

	testCases := []struct {
		name      string
		original  string
		ratio     float64
		effective bool
	}{
		{"highly compressible", "aaaaaaaaaa", 0.5, true},
		{"barely compressible", "abcdefghij", 0.89, true},
		{"not compressible", "abcdefghij", 0.91, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			original := []byte(tc.original)
			// Create compressed data with desired ratio
			compSize := int(float64(len(original)) * tc.ratio)
			compressed := make([]byte, compSize)

			effective := comp.IsCompressionEffective(original, compressed)
			if effective != tc.effective {
				t.Errorf("Expected %v, got %v for ratio %.2f", tc.effective, effective, tc.ratio)
			}
		})
	}
}

// TestCompressStream tests streaming compression
func TestCompressStream(t *testing.T) {
	comp := storage.NewCompressor()
	originalData := []byte("Stream compression test data. " +
		"This tests compression from a reader to a writer.")

	reader := bytes.NewReader(originalData)
	writer := &bytes.Buffer{}

	written, err := comp.CompressStream(reader, writer)
	if err != nil {
		t.Fatalf("CompressStream failed: %v", err)
	}

	if written <= 0 {
		t.Errorf("Expected positive bytes written, got %d", written)
	}

	if writer.Len() == 0 {
		t.Error("Compressed writer is empty")
	}
}

// TestDecompressStream tests streaming decompression
func TestDecompressStream(t *testing.T) {
	comp := storage.NewCompressor()
	originalData := []byte("Stream decompression test. This verifies the decompression stream works correctly.")

	// First, compress the data
	compressed, err := comp.Compress(originalData)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	// Then decompress using stream
	reader := bytes.NewReader(compressed)
	writer := &bytes.Buffer{}

	written, err := comp.DecompressStream(reader, writer)
	if err != nil {
		t.Fatalf("DecompressStream failed: %v", err)
	}

	if written <= 0 {
		t.Errorf("Expected positive bytes written, got %d", written)
	}

	decompressed := writer.Bytes()
	if !bytes.Equal(decompressed, originalData) {
		t.Errorf("Decompressed data doesn't match original")
	}
}

// TestLargeDataCompression tests compression with large data
func TestLargeDataCompression(t *testing.T) {
	comp := storage.NewCompressor()

	// Create 1MB of data
	originalData := bytes.Repeat([]byte("The quick brown fox jumps over the lazy dog. "), 20000)

	compressed, err := comp.Compress(originalData)
	if err != nil {
		t.Fatalf("Compress large data failed: %v", err)
	}

	// Verify compression is effective
	ratio := comp.GetCompressionRatio(originalData, compressed)
	if ratio >= 1.0 {
		t.Errorf("Expected effective compression for repetitive data, got ratio %.2f", ratio)
	}

	// Verify decompression
	decompressed, err := comp.Decompress(compressed)
	if err != nil {
		t.Fatalf("Decompress large data failed: %v", err)
	}

	if !bytes.Equal(decompressed, originalData) {
		t.Error("Decompressed large data doesn't match original")
	}
}

// TestCompressionWithBinaryData tests compression with binary data
func TestCompressionWithBinaryData(t *testing.T) {
	comp := storage.NewCompressor()

	// Create binary data
	originalData := make([]byte, 1000)
	for i := 0; i < len(originalData); i++ {
		originalData[i] = byte(i % 256)
	}

	compressed, err := comp.Compress(originalData)
	if err != nil {
		t.Fatalf("Compress binary data failed: %v", err)
	}

	decompressed, err := comp.Decompress(compressed)
	if err != nil {
		t.Fatalf("Decompress binary data failed: %v", err)
	}

	if !bytes.Equal(decompressed, originalData) {
		t.Error("Decompressed binary data doesn't match original")
	}
}
