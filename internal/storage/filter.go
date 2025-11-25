package storage

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/bits-and-blooms/bloom/v3"
)

// BloomFilter represents a probabilistic set membership test for space-efficient filtering
type BloomFilter interface {
	// Add adds an element to the bloom filter
	Add(elem string)

	// Contains returns true if the element might be in the set (with some false positives)
	Contains(elem string) bool

	// FalsePositiveRate returns the configured false positive rate
	FalsePositiveRate() float32

	// Serialize returns the binary representation of the bloom filter
	Serialize() ([]byte, error)

	// Deserialize populates the bloom filter from binary representation
	Deserialize(data []byte) error

	// MarshalJSON implements JSON marshaling for storage
	MarshalJSON() ([]byte, error)

	// UnmarshalJSON implements JSON unmarshaling for storage
	UnmarshalJSON(data []byte) error
}

// StandardBloomFilter implements BloomFilter using bits-and-blooms/bloom
type StandardBloomFilter struct {
	filter             *bloom.BloomFilter
	falsePositiveRate  float32
	expectedElements   uint
	serializedBitset   []byte
	hashFunctions      uint
}

// NewBloomFilter creates a new bloom filter with specified expected elements and FP rate
func NewBloomFilter(expectedElements uint, falsePositiveRate float32) *StandardBloomFilter {
	// bits-and-blooms uses estimated false positive rate to calculate optimal parameters
	filter := bloom.NewWithEstimates(expectedElements, float64(falsePositiveRate))

	hashFunctions := calculateHashFunctions(falsePositiveRate)

	return &StandardBloomFilter{
		filter:            filter,
		falsePositiveRate: falsePositiveRate,
		expectedElements:  expectedElements,
		hashFunctions:     hashFunctions,
	}
}

// Add adds an element to the bloom filter
func (b *StandardBloomFilter) Add(elem string) {
	b.filter.AddString(elem)
}

// Contains returns true if element might be in the set
func (b *StandardBloomFilter) Contains(elem string) bool {
	return b.filter.ContainsString(elem)
}

// FalsePositiveRate returns the configured false positive rate
func (b *StandardBloomFilter) FalsePositiveRate() float32 {
	return b.falsePositiveRate
}

// Serialize returns the binary representation of the bloom filter
func (b *StandardBloomFilter) Serialize() ([]byte, error) {
	return b.filter.MarshalJSON()
}

// Deserialize populates the bloom filter from binary representation
func (b *StandardBloomFilter) Deserialize(data []byte) error {
	newFilter := &bloom.BloomFilter{}
	if err := newFilter.UnmarshalJSON(data); err != nil {
		return fmt.Errorf("failed to deserialize bloom filter: %w", err)
	}
	b.filter = newFilter
	b.serializedBitset = data
	return nil
}

// MarshalJSON implements JSON marshaling for storage
func (b *StandardBloomFilter) MarshalJSON() ([]byte, error) {
	serialized, err := b.Serialize()
	if err != nil {
		return nil, err
	}

	// Encode bitset as base64 for JSON storage
	bitsetBase64 := base64.StdEncoding.EncodeToString(serialized)

	jsonData := map[string]interface{}{
		"size_bits":               b.filter.BitSet().Len(),
		"hash_functions":          b.hashFunctions,
		"bitset":                  bitsetBase64,
		"false_positive_rate":     b.falsePositiveRate,
		"expected_elements":       b.expectedElements,
	}

	return json.Marshal(jsonData)
}

// UnmarshalJSON implements JSON unmarshaling for storage
func (b *StandardBloomFilter) UnmarshalJSON(data []byte) error {
	var jsonData map[string]interface{}
	if err := json.Unmarshal(data, &jsonData); err != nil {
		return fmt.Errorf("failed to unmarshal bloom filter JSON: %w", err)
	}

	// Extract base64-encoded bitset
	bitsetBase64, ok := jsonData["bitset"].(string)
	if !ok {
		return fmt.Errorf("bitset field missing or invalid type")
	}

	bitsetData, err := base64.StdEncoding.DecodeString(bitsetBase64)
	if err != nil {
		return fmt.Errorf("failed to decode base64 bitset: %w", err)
	}

	// Extract false positive rate
	fpRate, ok := jsonData["false_positive_rate"].(float64)
	if !ok {
		return fmt.Errorf("false_positive_rate field missing or invalid type")
	}
	b.falsePositiveRate = float32(fpRate)

	// Extract hash functions count
	if hashFuncs, ok := jsonData["hash_functions"].(float64); ok {
		b.hashFunctions = uint(hashFuncs)
	}

	// Extract expected elements
	if expectedElems, ok := jsonData["expected_elements"].(float64); ok {
		b.expectedElements = uint(expectedElems)
	}

	// Deserialize the bloom filter
	return b.Deserialize(bitsetData)
}

// calculateHashFunctions returns the recommended number of hash functions for a given FP rate
// Based on optimal formula: k = -log2(FP_rate)
func calculateHashFunctions(fpRate float32) uint {
	// Approximate formula for reasonable values
	// 5% -> 5 functions, 3% -> 5 functions, 1% -> 7 functions
	if fpRate <= 0.01 {
		return 7
	} else if fpRate <= 0.05 {
		return 5
	} else if fpRate <= 0.10 {
		return 4
	}
	return 3
}
