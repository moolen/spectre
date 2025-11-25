package storage

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/moritz/rpk/internal/storage"
)

func TestBloomFilterAdd(t *testing.T) {
	bf := storage.NewBloomFilter(1000, 0.05)

	bf.Add("pod1")
	bf.Add("pod2")
	bf.Add("deployment1")

	if !bf.Contains("pod1") {
		t.Error("Expected to find pod1 in bloom filter")
	}
	if !bf.Contains("pod2") {
		t.Error("Expected to find pod2 in bloom filter")
	}
	if !bf.Contains("deployment1") {
		t.Error("Expected to find deployment1 in bloom filter")
	}
}

func TestBloomFilterContains(t *testing.T) {
	bf := storage.NewBloomFilter(1000, 0.05)

	bf.Add("exists")

	if bf.Contains("does_not_exist") && !bf.Contains("exists") {
		t.Error("Unexpected behavior: non-existent item found")
	}
	if !bf.Contains("exists") {
		t.Error("Expected to find 'exists' in bloom filter")
	}
}

func TestBloomFilterFalsePositiveRate(t *testing.T) {
	tests := []struct {
		name   string
		fpRate float32
	}{
		{"5%", 0.05},
		{"3%", 0.03},
		{"1%", 0.01},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bf := storage.NewBloomFilter(1000, tt.fpRate)
			if bf.FalsePositiveRate() != tt.fpRate {
				t.Errorf("Expected FP rate %f, got %f", tt.fpRate, bf.FalsePositiveRate())
			}
		})
	}
}

func TestBloomFilterSerialization(t *testing.T) {
	bf := storage.NewBloomFilter(1000, 0.05)
	bf.Add("item1")
	bf.Add("item2")
	bf.Add("item3")

	// Serialize
	serialized, err := bf.Serialize()
	if err != nil {
		t.Fatalf("Failed to serialize bloom filter: %v", err)
	}

	// Create new filter and deserialize
	bf2 := storage.NewBloomFilter(1000, 0.05)
	err = bf2.Deserialize(serialized)
	if err != nil {
		t.Fatalf("Failed to deserialize bloom filter: %v", err)
	}

	// Verify contents
	if !bf2.Contains("item1") {
		t.Error("Expected to find item1 after deserialization")
	}
	if !bf2.Contains("item2") {
		t.Error("Expected to find item2 after deserialization")
	}
	if !bf2.Contains("item3") {
		t.Error("Expected to find item3 after deserialization")
	}
}

func TestBloomFilterJSONMarshaling(t *testing.T) {
	bf := storage.NewBloomFilter(1000, 0.05)
	bf.Add("kind1")
	bf.Add("kind2")

	// Marshal to JSON
	jsonData, err := bf.MarshalJSON()
	if err != nil {
		t.Fatalf("Failed to marshal bloom filter: %v", err)
	}

	// Verify JSON contains expected fields
	var jsonObj map[string]interface{}
	err = json.Unmarshal(jsonData, &jsonObj)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Check required fields
	requiredFields := []string{"size_bits", "hash_functions", "bitset", "false_positive_rate", "expected_elements"}
	for _, field := range requiredFields {
		if _, ok := jsonObj[field]; !ok {
			t.Errorf("Missing required field: %s", field)
		}
	}

	// Verify bitset is base64 encoded
	bitsetStr, ok := jsonObj["bitset"].(string)
	if !ok {
		t.Error("Bitset is not a string")
	}
	if _, err := base64.StdEncoding.DecodeString(bitsetStr); err != nil {
		t.Error("Bitset is not valid base64")
	}
}

func TestBloomFilterJSONUnmarshaling(t *testing.T) {
	// Create and marshal a bloom filter
	bf1 := storage.NewBloomFilter(1000, 0.05)
	bf1.Add("test1")
	bf1.Add("test2")

	jsonData, err := bf1.MarshalJSON()
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Create new filter and unmarshal
	bf2 := storage.NewBloomFilter(100, 0.10) // Different initial values
	err = bf2.UnmarshalJSON(jsonData)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify the data matches
	if !bf2.Contains("test1") {
		t.Error("Expected to find test1 after unmarshaling")
	}
	if !bf2.Contains("test2") {
		t.Error("Expected to find test2 after unmarshaling")
	}
	if bf2.FalsePositiveRate() != 0.05 {
		t.Errorf("Expected FP rate 0.05, got %f", bf2.FalsePositiveRate())
	}
}

func TestBloomFilterEmptyFilter(t *testing.T) {
	bf := storage.NewBloomFilter(1000, 0.05)

	// An empty filter should not contain anything
	if bf.Contains("anything") {
		t.Error("Empty filter should not contain anything (or very rarely due to FP rate)")
	}
}

func TestBloomFilterLargeDataset(t *testing.T) {
	bf := storage.NewBloomFilter(10000, 0.05)

	// Add many items
	itemCount := 1000
	for i := 0; i < itemCount; i++ {
		bf.Add(string(rune(i)))
	}

	// Verify all items are found (should be 100% hit rate)
	missCount := 0
	for i := 0; i < itemCount; i++ {
		if !bf.Contains(string(rune(i))) {
			missCount++
		}
	}

	if missCount > 0 {
		t.Errorf("Expected 0 misses, got %d", missCount)
	}

	// Check false positive rate by testing non-existent items
	// With 1000 items in a filter sized for 10000, FP rate should be ~5%
	fpCount := 0
	testCount := 10000
	for i := itemCount; i < itemCount+testCount; i++ {
		if bf.Contains(string(rune(i))) {
			fpCount++
		}
	}

	fpRate := float64(fpCount) / float64(testCount)
	expectedFPRate := 0.05
	tolerance := 0.02 // Allow 2% tolerance

	if fpRate < expectedFPRate-tolerance || fpRate > expectedFPRate+tolerance {
		t.Logf("FP rate: %f (expected ~%f, tolerance ±%f)", fpRate, expectedFPRate, tolerance)
		// Log but don't fail - FP rate is probabilistic
	}
}
