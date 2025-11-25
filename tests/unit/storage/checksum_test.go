package storage

import (
	"testing"

	"github.com/moritz/rpk/internal/storage"
)

func TestComputeChecksum(t *testing.T) {
	data1 := []byte("test data 1")
	data2 := []byte("test data 2")
	data1Again := []byte("test data 1")

	checksum1 := storage.ComputeChecksum(data1)
	checksum2 := storage.ComputeChecksum(data2)
	checksum1Again := storage.ComputeChecksum(data1Again)

	// Same data should produce same checksum
	if checksum1 != checksum1Again {
		t.Errorf("Same data produced different checksums: %s vs %s", checksum1, checksum1Again)
	}

	// Different data should produce different checksums
	if checksum1 == checksum2 {
		t.Errorf("Different data produced same checksum")
	}

	// Checksum should be a valid hex string (32 characters for MD5)
	if len(checksum1) != 32 {
		t.Errorf("Expected 32-char MD5 checksum, got %d chars: %s", len(checksum1), checksum1)
	}
}

func TestChecksumDeterministic(t *testing.T) {
	data := []byte("The quick brown fox jumps over the lazy dog")

	// Compute checksum 3 times
	checksum1 := storage.ComputeChecksum(data)
	checksum2 := storage.ComputeChecksum(data)
	checksum3 := storage.ComputeChecksum(data)

	if checksum1 != checksum2 || checksum2 != checksum3 {
		t.Errorf("Checksums are not deterministic: %s, %s, %s", checksum1, checksum2, checksum3)
	}
}

func TestChecksumSensitivity(t *testing.T) {
	data1 := []byte("Hello")
	data2 := []byte("hello") // Different case

	checksum1 := storage.ComputeChecksum(data1)
	checksum2 := storage.ComputeChecksum(data2)

	if checksum1 == checksum2 {
		t.Errorf("Different data (different case) produced same checksum")
	}
}

func TestChecksumEmptyData(t *testing.T) {
	data := []byte("")
	checksum := storage.ComputeChecksum(data)

	if checksum == "" {
		t.Errorf("Expected non-empty checksum for empty data")
	}

	if len(checksum) != 32 {
		t.Errorf("Expected 32-char MD5 for empty data, got %d chars", len(checksum))
	}
}

func TestChecksumLargeData(t *testing.T) {
	// Create large data (1MB)
	data := make([]byte, 1024*1024)
	for i := range data {
		data[i] = byte(i % 256)
	}

	checksum := storage.ComputeChecksum(data)

	if len(checksum) != 32 {
		t.Errorf("Expected 32-char MD5 for large data, got %d chars", len(checksum))
	}

	// Verify determinism
	checksum2 := storage.ComputeChecksum(data)
	if checksum != checksum2 {
		t.Errorf("Large data checksum not deterministic")
	}
}
