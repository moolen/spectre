package storage

import (
	"bytes"
	"fmt"
	"io"

	"github.com/klauspost/compress/gzip"
	"github.com/moolen/spectre/internal/logging"
)

// Compressor handles compression and decompression of event data
type Compressor struct {
	logger *logging.Logger
}

// NewCompressor creates a new compressor
func NewCompressor() *Compressor {
	return &Compressor{
		logger: logging.GetLogger("compressor"),
	}
}

// Compress compresses data using gzip
func (c *Compressor) Compress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return []byte{}, nil
	}

	var buf bytes.Buffer

	// Create gzip writer
	writer, err := gzip.NewWriterLevel(&buf, gzip.DefaultCompression)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip writer: %w", err)
	}

	// Write data to gzip writer
	if _, err := writer.Write(data); err != nil {
		_ = writer.Close()
		return nil, fmt.Errorf("failed to write data to gzip: %w", err)
	}

	// Close the writer to flush
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// Decompress decompresses gzip data
func (c *Compressor) Decompress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return []byte{}, nil
	}

	// Create gzip reader
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			// Log error but don't fail the operation
		}
	}()

	// Read decompressed data
	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read decompressed data: %w", err)
	}

	return decompressed, nil
}

// GetCompressionRatio calculates the compression ratio
func (c *Compressor) GetCompressionRatio(original, compressed []byte) float64 {
	if len(original) == 0 {
		return 0.0
	}
	return float64(len(compressed)) / float64(len(original))
}

// GetCompressionSavings calculates the bytes saved
func (c *Compressor) GetCompressionSavings(original, compressed []byte) int64 {
	return int64(len(original) - len(compressed))
}

// IsCompressionEffective checks if compression is effective (at least 10% reduction)
func (c *Compressor) IsCompressionEffective(original, compressed []byte) bool {
	if len(original) == 0 {
		return false
	}
	ratio := c.GetCompressionRatio(original, compressed)
	return ratio < 0.9 // More than 10% reduction
}

// CompressStream compresses data from a reader and writes to a writer
func (c *Compressor) CompressStream(reader io.Reader, writer io.Writer) (int64, error) {
	gzipWriter, err := gzip.NewWriterLevel(writer, gzip.DefaultCompression)
	if err != nil {
		return 0, fmt.Errorf("failed to create gzip writer: %w", err)
	}
	defer func() {
		if err := gzipWriter.Close(); err != nil {
			// Log error but don't fail the operation
		}
	}()

	written, err := io.Copy(gzipWriter, reader)
	if err != nil {
		return 0, fmt.Errorf("failed to compress stream: %w", err)
	}

	return written, nil
}

// DecompressStream decompresses gzipped data from a reader and writes to a writer
func (c *Compressor) DecompressStream(reader io.Reader, writer io.Writer) (int64, error) {
	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return 0, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer func() {
		if err := gzipReader.Close(); err != nil {
			// Log error but don't fail the operation
		}
	}()

	written, err := io.Copy(writer, gzipReader)
	if err != nil {
		return 0, fmt.Errorf("failed to decompress stream: %w", err)
	}

	return written, nil
}
