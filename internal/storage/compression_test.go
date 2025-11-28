package storage

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestNewCompressor(t *testing.T) {
	compressor := NewCompressor()
	if compressor == nil {
		t.Fatal("expected non-nil compressor")
	}
	if compressor.logger == nil {
		t.Error("expected logger to be initialized")
	}
}

func TestCompressDecompress(t *testing.T) {
	compressor := NewCompressor()

	original := []byte("This is test data that should compress well. " +
		"This is test data that should compress well. " +
		"This is test data that should compress well. " +
		"This is test data that should compress well.")

	compressed, err := compressor.Compress(original)
	if err != nil {
		t.Fatalf("failed to compress: %v", err)
	}

	if len(compressed) == 0 {
		t.Error("expected compressed data")
	}

	decompressed, err := compressor.Decompress(compressed)
	if err != nil {
		t.Fatalf("failed to decompress: %v", err)
	}

	if !bytes.Equal(original, decompressed) {
		t.Error("decompressed data does not match original")
	}
}

func TestCompressEmpty(t *testing.T) {
	compressor := NewCompressor()

	compressed, err := compressor.Compress([]byte{})
	if err != nil {
		t.Fatalf("failed to compress empty data: %v", err)
	}

	if len(compressed) != 0 {
		t.Errorf("expected empty compressed data, got %d bytes", len(compressed))
	}
}

func TestDecompressEmpty(t *testing.T) {
	compressor := NewCompressor()

	decompressed, err := compressor.Decompress([]byte{})
	if err != nil {
		t.Fatalf("failed to decompress empty data: %v", err)
	}

	if len(decompressed) != 0 {
		t.Errorf("expected empty decompressed data, got %d bytes", len(decompressed))
	}
}

func TestCompressLargeData(t *testing.T) {
	compressor := NewCompressor()

	// Create 1MB of data
	original := make([]byte, 1024*1024)
	for i := range original {
		original[i] = byte(i % 256)
	}

	compressed, err := compressor.Compress(original)
	if err != nil {
		t.Fatalf("failed to compress large data: %v", err)
	}

	if len(compressed) == 0 {
		t.Error("expected compressed data")
	}

	decompressed, err := compressor.Decompress(compressed)
	if err != nil {
		t.Fatalf("failed to decompress large data: %v", err)
	}

	if len(decompressed) != len(original) {
		t.Errorf("decompressed size mismatch: expected %d, got %d", len(original), len(decompressed))
	}

	if !bytes.Equal(original, decompressed) {
		t.Error("decompressed data does not match original")
	}
}

func TestGetCompressionRatio(t *testing.T) {
	compressor := NewCompressor()

	original := []byte("test data")
	compressed := []byte("compressed")

	ratio := compressor.GetCompressionRatio(original, compressed)
	if ratio != float64(len(compressed))/float64(len(original)) {
		t.Errorf("unexpected compression ratio: %f", ratio)
	}
}

func TestGetCompressionRatioEmpty(t *testing.T) {
	compressor := NewCompressor()

	ratio := compressor.GetCompressionRatio([]byte{}, []byte{})
	if ratio != 0.0 {
		t.Errorf("expected 0.0 for empty data, got %f", ratio)
	}
}

func TestGetCompressionSavings(t *testing.T) {
	compressor := NewCompressor()

	original := []byte("test data")
	compressed := []byte("comp")

	savings := compressor.GetCompressionSavings(original, compressed)
	expected := int64(len(original) - len(compressed))
	if savings != expected {
		t.Errorf("expected savings %d, got %d", expected, savings)
	}
}

func TestIsCompressionEffective(t *testing.T) {
	compressor := NewCompressor()

	original := []byte("test data that is longer")
	compressed := []byte("small")

	if !compressor.IsCompressionEffective(original, compressed) {
		t.Error("expected compression to be effective")
	}

	// Test with ratio > 0.9 (not effective)
	compressed = make([]byte, len(original))
	copy(compressed, original)
	if compressor.IsCompressionEffective(original, compressed) {
		t.Error("expected compression to not be effective")
	}
}

func TestIsCompressionEffectiveEmpty(t *testing.T) {
	compressor := NewCompressor()

	if compressor.IsCompressionEffective([]byte{}, []byte{}) {
		t.Error("expected compression to not be effective for empty data")
	}
}

func TestCompressStream(t *testing.T) {
	compressor := NewCompressor()

	original := []byte("This is test data for stream compression. " +
		"This is test data for stream compression. " +
		"This is test data for stream compression.")

	reader := bytes.NewReader(original)
	var writer bytes.Buffer

	written, err := compressor.CompressStream(reader, &writer)
	if err != nil {
		t.Fatalf("failed to compress stream: %v", err)
	}

	if written == 0 {
		t.Error("expected bytes to be written")
	}

	if writer.Len() == 0 {
		t.Error("expected compressed data in buffer")
	}
}

func TestDecompressStream(t *testing.T) {
	compressor := NewCompressor()

	original := []byte("This is test data for stream decompression. " +
		"This is test data for stream decompression. " +
		"This is test data for stream decompression.")

	// First compress
	compressed, err := compressor.Compress(original)
	if err != nil {
		t.Fatalf("failed to compress: %v", err)
	}

	// Then decompress via stream
	reader := bytes.NewReader(compressed)
	var writer bytes.Buffer

	written, err := compressor.DecompressStream(reader, &writer)
	if err != nil {
		t.Fatalf("failed to decompress stream: %v", err)
	}

	if written == 0 {
		t.Error("expected bytes to be written")
	}

	if !bytes.Equal(original, writer.Bytes()) {
		t.Error("decompressed stream data does not match original")
	}
}

func TestCompressStreamLarge(t *testing.T) {
	compressor := NewCompressor()

	// Create 100KB of data
	original := make([]byte, 100*1024)
	for i := range original {
		original[i] = byte(i % 256)
	}

	reader := bytes.NewReader(original)
	var writer bytes.Buffer

	written, err := compressor.CompressStream(reader, &writer)
	if err != nil {
		t.Fatalf("failed to compress large stream: %v", err)
	}

	if written == 0 {
		t.Error("expected bytes to be written")
	}

	// Decompress and verify
	var decompressed bytes.Buffer
	compressedReader := bytes.NewReader(writer.Bytes())
	_, err = compressor.DecompressStream(compressedReader, &decompressed)
	if err != nil {
		t.Fatalf("failed to decompress: %v", err)
	}

	if !bytes.Equal(original, decompressed.Bytes()) {
		t.Error("decompressed data does not match original")
	}
}

func TestCompressDecompressRoundTrip(t *testing.T) {
	compressor := NewCompressor()

	testCases := []string{
		"simple",
		"data with spaces",
		"data\nwith\nnewlines",
		"data with special chars: !@#$%^&*()",
		strings.Repeat("repeated data ", 100),
	}

	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			original := []byte(tc)

			compressed, err := compressor.Compress(original)
			if err != nil {
				t.Fatalf("failed to compress: %v", err)
			}

			decompressed, err := compressor.Decompress(compressed)
			if err != nil {
				t.Fatalf("failed to decompress: %v", err)
			}

			if !bytes.Equal(original, decompressed) {
				t.Errorf("round trip failed for: %q", tc)
			}
		})
	}
}

func TestCompressStreamErrorHandling(t *testing.T) {
	compressor := NewCompressor()

	// Test with a reader that returns an error
	errorReader := &errorReader{err: io.ErrClosedPipe}
	var writer bytes.Buffer

	_, err := compressor.CompressStream(errorReader, &writer)
	if err == nil {
		t.Error("expected error from error reader")
	}
}

func TestDecompressStreamErrorHandling(t *testing.T) {
	compressor := NewCompressor()

	// Test with invalid gzip data
	invalidData := []byte("not valid gzip data")
	reader := bytes.NewReader(invalidData)
	var writer bytes.Buffer

	_, err := compressor.DecompressStream(reader, &writer)
	if err == nil {
		t.Error("expected error for invalid gzip data")
	}
}

// errorReader is a test helper that always returns an error
type errorReader struct {
	err error
}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, e.err
}
