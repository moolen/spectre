package logging

import (
	"testing"
)

// TestCloneFields_NilInput tests cloning a nil map
func TestCloneFields_NilInput(t *testing.T) {
	result := cloneFields(nil)
	if result == nil {
		t.Error("Expected non-nil map, got nil")
	}
	if len(result) != 0 {
		t.Errorf("Expected empty map, got length %d", len(result))
	}
}

// TestCloneFields_EmptyMap tests cloning an empty map
func TestCloneFields_EmptyMap(t *testing.T) {
	src := make(map[string]interface{})
	result := cloneFields(src)

	if result == nil {
		t.Error("Expected non-nil map, got nil")
	}
	if len(result) != 0 {
		t.Errorf("Expected empty map, got length %d", len(result))
	}
}

// TestCloneFields_SingleField tests cloning a map with one field
func TestCloneFields_SingleField(t *testing.T) {
	src := map[string]interface{}{
		"key1": "value1",
	}

	result := cloneFields(src)

	if len(result) != 1 {
		t.Errorf("Expected 1 field, got %d", len(result))
	}
	if result["key1"] != "value1" {
		t.Errorf("Expected 'value1', got %v", result["key1"])
	}
}

// TestCloneFields_MultipleFields tests cloning a map with multiple fields
func TestCloneFields_MultipleFields(t *testing.T) {
	src := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
		"key3": true,
		"key4": nil,
	}

	result := cloneFields(src)

	if len(result) != 4 {
		t.Errorf("Expected 4 fields, got %d", len(result))
	}

	// Verify all fields copied correctly
	if result["key1"] != "value1" {
		t.Errorf("key1: expected 'value1', got %v", result["key1"])
	}
	if result["key2"] != 42 {
		t.Errorf("key2: expected 42, got %v", result["key2"])
	}
	if result["key3"] != true {
		t.Errorf("key3: expected true, got %v", result["key3"])
	}
	if result["key4"] != nil {
		t.Errorf("key4: expected nil, got %v", result["key4"])
	}
}

// TestCloneFields_Independence tests that cloned map is independent
func TestCloneFields_Independence(t *testing.T) {
	src := map[string]interface{}{
		"key1": "original",
	}

	result := cloneFields(src)

	// Mutate the result
	result["key1"] = "modified"
	result["key2"] = "added"

	// Verify source is unaffected
	if src["key1"] != "original" {
		t.Errorf("Source was modified: expected 'original', got %v", src["key1"])
	}
	if _, exists := src["key2"]; exists {
		t.Error("Source was modified: unexpected key2")
	}

	// Verify result has changes
	if result["key1"] != "modified" {
		t.Errorf("Result: expected 'modified', got %v", result["key1"])
	}
	if result["key2"] != "added" {
		t.Errorf("Result: expected 'added', got %v", result["key2"])
	}
}

// TestCloneFields_LargeMap tests cloning a large map for performance
func TestCloneFields_LargeMap(t *testing.T) {
	src := make(map[string]interface{})
	for i := 0; i < 1000; i++ {
		src[string(rune(i))] = i
	}

	result := cloneFields(src)

	if len(result) != len(src) {
		t.Errorf("Expected %d fields, got %d", len(src), len(result))
	}

	// Spot check a few values
	if result[string(rune(0))] != 0 {
		t.Error("Field 0 not cloned correctly")
	}
	if result[string(rune(500))] != 500 {
		t.Error("Field 500 not cloned correctly")
	}
	if result[string(rune(999))] != 999 {
		t.Error("Field 999 not cloned correctly")
	}
}

// BenchmarkCloneFields_Small benchmarks cloning a small map
func BenchmarkCloneFields_Small(b *testing.B) {
	src := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cloneFields(src)
	}
}

// BenchmarkCloneFields_Medium benchmarks cloning a medium-sized map
func BenchmarkCloneFields_Medium(b *testing.B) {
	src := make(map[string]interface{})
	for i := 0; i < 10; i++ {
		src[string(rune(i))] = i
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cloneFields(src)
	}
}

// BenchmarkCloneFields_Large benchmarks cloning a large map
func BenchmarkCloneFields_Large(b *testing.B) {
	src := make(map[string]interface{})
	for i := 0; i < 100; i++ {
		src[string(rune(i))] = i
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cloneFields(src)
	}
}
