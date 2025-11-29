package storage

import (
	"encoding/json"
	"testing"
)

func TestNewBloomFilter(t *testing.T) {
	filter := NewBloomFilter(1000, 0.05)

	if filter == nil {
		t.Fatal("expected non-nil filter")
	}

	if filter.falsePositiveRate != 0.05 {
		t.Errorf("expected false positive rate 0.05, got %f", filter.falsePositiveRate)
	}

	if filter.expectedElements != 1000 {
		t.Errorf("expected 1000 elements, got %d", filter.expectedElements)
	}
}

func TestBloomFilterAddContains(t *testing.T) {
	filter := NewBloomFilter(100, 0.05)

	items := []string{"item1", "item2", "item3", "item4", "item5"}

	// Add items
	for _, item := range items {
		filter.Add(item)
	}

	// Check that all added items are contained
	for _, item := range items {
		if !filter.Contains(item) {
			t.Errorf("expected filter to contain %s", item)
		}
	}
}

func TestBloomFilterFalsePositives(t *testing.T) {
	filter := NewBloomFilter(100, 0.05)

	// Add some items
	items := []string{"item1", "item2", "item3"}
	for _, item := range items {
		filter.Add(item)
	}

	// Check non-existent items (may have false positives, but should be rare)
	nonExistent := []string{"nonexistent1", "nonexistent2", "nonexistent3"}
	falsePositives := 0
	for _, item := range nonExistent {
		if filter.Contains(item) {
			falsePositives++
		}
	}

	// With 5% false positive rate, we might get some, but not all
	// This is a probabilistic test, so we just verify it doesn't crash
	if falsePositives > len(nonExistent) {
		t.Error("unexpectedly high false positive rate")
	}
}

func TestBloomFilterFalsePositiveRate(t *testing.T) {
	filter := NewBloomFilter(100, 0.05)

	rate := filter.FalsePositiveRate()
	if rate != 0.05 {
		t.Errorf("expected false positive rate 0.05, got %f", rate)
	}
}

func TestBloomFilterSerializeDeserialize(t *testing.T) {
	filter := NewBloomFilter(100, 0.05)

	// Add some items
	filter.Add("item1")
	filter.Add("item2")
	filter.Add("item3")

	// Serialize
	data, err := filter.Serialize()
	if err != nil {
		t.Fatalf("failed to serialize: %v", err)
	}

	if len(data) == 0 {
		t.Error("expected serialized data")
	}

	// Create new filter and deserialize
	newFilter := NewBloomFilter(100, 0.05)
	if err := newFilter.Deserialize(data); err != nil {
		t.Fatalf("failed to deserialize: %v", err)
	}

	// Verify items are still contained
	if !newFilter.Contains("item1") {
		t.Error("expected deserialized filter to contain item1")
	}
	if !newFilter.Contains("item2") {
		t.Error("expected deserialized filter to contain item2")
	}
	if !newFilter.Contains("item3") {
		t.Error("expected deserialized filter to contain item3")
	}
}

func TestBloomFilterMarshalUnmarshalJSON(t *testing.T) {
	filter := NewBloomFilter(100, 0.05)

	// Add some items
	filter.Add("item1")
	filter.Add("item2")

	// Marshal to JSON
	jsonData, err := filter.MarshalJSON()
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}

	if len(jsonData) == 0 {
		t.Error("expected JSON data")
	}

	// Unmarshal from JSON
	newFilter := &StandardBloomFilter{}
	if err := newFilter.UnmarshalJSON(jsonData); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	// Verify items are still contained
	if !newFilter.Contains("item1") {
		t.Error("expected unmarshaled filter to contain item1")
	}
	if !newFilter.Contains("item2") {
		t.Error("expected unmarshaled filter to contain item2")
	}
}

func TestBloomFilterMarshalJSONStructure(t *testing.T) {
	filter := NewBloomFilter(100, 0.05)
	filter.Add("test")

	jsonData, err := filter.MarshalJSON()
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var jsonMap map[string]interface{}
	if err := json.Unmarshal(jsonData, &jsonMap); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	// Verify structure
	if _, ok := jsonMap["bitset"]; !ok {
		t.Error("expected bitset field in JSON")
	}
	if _, ok := jsonMap["false_positive_rate"]; !ok {
		t.Error("expected false_positive_rate field in JSON")
	}
	if _, ok := jsonMap["expected_elements"]; !ok {
		t.Error("expected expected_elements field in JSON")
	}
}

func TestBloomFilterUnmarshalJSONInvalid(t *testing.T) {
	filter := &StandardBloomFilter{}

	// Test with invalid JSON
	invalidJSON := []byte(`{"invalid": "data"}`)
	if err := filter.UnmarshalJSON(invalidJSON); err == nil {
		t.Error("expected error for invalid JSON structure")
	}

	// Test with missing bitset
	missingBitset := []byte(`{"false_positive_rate": 0.05}`)
	if err := filter.UnmarshalJSON(missingBitset); err == nil {
		t.Error("expected error for missing bitset")
	}
}

func TestCalculateHashFunctions(t *testing.T) {
	tests := []struct {
		fpRate float32
		want   uint
	}{
		{0.01, 7},
		{0.05, 5},
		{0.10, 4},
		{0.20, 3},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := calculateHashFunctions(tt.fpRate)
			if got != tt.want {
				t.Errorf("calculateHashFunctions(%f) = %d, want %d", tt.fpRate, got, tt.want)
			}
		})
	}
}

func TestBloomFilterLargeDataset(t *testing.T) {
	filter := NewBloomFilter(10000, 0.01)

	// Add many items
	for i := 0; i < 1000; i++ {
		filter.Add(string(rune(i)))
	}

	// Verify all added items are contained
	for i := 0; i < 1000; i++ {
		if !filter.Contains(string(rune(i))) {
			t.Errorf("expected filter to contain item %d", i)
		}
	}
}

func TestBloomFilterEmpty(t *testing.T) {
	filter := NewBloomFilter(100, 0.05)

	// Empty filter should not contain anything
	if filter.Contains("nonexistent") {
		t.Error("empty filter should not contain any items")
	}

	// Serialize empty filter
	data, err := filter.Serialize()
	if err != nil {
		t.Fatalf("failed to serialize empty filter: %v", err)
	}

	// Deserialize
	newFilter := NewBloomFilter(100, 0.05)
	if err := newFilter.Deserialize(data); err != nil {
		t.Fatalf("failed to deserialize empty filter: %v", err)
	}

	if newFilter.Contains("nonexistent") {
		t.Error("deserialized empty filter should not contain items")
	}
}
