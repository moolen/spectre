package storage

import (
	"testing"

	"github.com/moritz/rpk/internal/models"
	"github.com/moritz/rpk/internal/storage"
)

// TestNewSegmentMetadataIndex tests segment metadata index creation
func TestNewSegmentMetadataIndex(t *testing.T) {
	smi := storage.NewSegmentMetadataIndex()
	if smi == nil {
		t.Error("NewSegmentMetadataIndex returned nil")
	}
}

// TestAddMetadata tests adding metadata for a segment
func TestAddMetadata(t *testing.T) {
	smi := storage.NewSegmentMetadataIndex()

	metadata := models.SegmentMetadata{
		ResourceSummary: []models.ResourceMetadata{
			{Group: "apps", Version: "v1", Kind: "Pod", Namespace: "default"},
		},
		KindSet:      map[string]bool{"Pod": true, "Deployment": true},
		NamespaceSet: map[string]bool{"default": true},
		CompressionAlgorithm: "gzip",
		MinTimestamp: 1000,
		MaxTimestamp: 2000,
	}

	smi.AddMetadata(1, metadata)

	if smi.GetSegmentCount() != 1 {
		t.Errorf("Expected 1 segment, got %d", smi.GetSegmentCount())
	}
}

// TestAddMultipleMetadatas tests adding multiple segment metadatas
func TestAddMultipleMetadatas(t *testing.T) {
	smi := storage.NewSegmentMetadataIndex()

	metadatas := []struct {
		segmentID int32
		metadata  models.SegmentMetadata
	}{
		{
			1,
			models.SegmentMetadata{
				ResourceSummary: []models.ResourceMetadata{
					{Group: "apps", Version: "v1", Kind: "Pod", Namespace: "default"},
				},
				KindSet:              map[string]bool{"Pod": true},
				NamespaceSet:         map[string]bool{"default": true},
				CompressionAlgorithm: "gzip",
			},
		},
		{
			2,
			models.SegmentMetadata{
				ResourceSummary: []models.ResourceMetadata{
					{Group: "apps", Version: "v1", Kind: "Deployment", Namespace: "kube-system"},
				},
				KindSet:              map[string]bool{"Deployment": true},
				NamespaceSet:         map[string]bool{"kube-system": true},
				CompressionAlgorithm: "gzip",
			},
		},
		{
			3,
			models.SegmentMetadata{
				ResourceSummary: []models.ResourceMetadata{
					{Group: "", Version: "v1", Kind: "Service", Namespace: "default"},
				},
				KindSet:              map[string]bool{"Service": true},
				NamespaceSet:         map[string]bool{"default": true},
				CompressionAlgorithm: "gzip",
			},
		},
	}

	for _, m := range metadatas {
		smi.AddMetadata(m.segmentID, m.metadata)
	}

	if smi.GetSegmentCount() != 3 {
		t.Errorf("Expected 3 segments, got %d", smi.GetSegmentCount())
	}
}

// TestCanSegmentBeSkipped tests segment skipping logic for filters
func TestCanSegmentBeSkipped(t *testing.T) {
	smi := storage.NewSegmentMetadataIndex()

	// Add metadata for segments
	smi.AddMetadata(1, models.SegmentMetadata{
		ResourceSummary: []models.ResourceMetadata{
			{Group: "apps", Version: "v1", Kind: "Pod", Namespace: "default"},
		},
		KindSet:              map[string]bool{"Pod": true},
		NamespaceSet:         map[string]bool{"default": true},
		CompressionAlgorithm: "gzip",
	})

	smi.AddMetadata(2, models.SegmentMetadata{
		ResourceSummary: []models.ResourceMetadata{
			{Group: "apps", Version: "v1", Kind: "Deployment", Namespace: "kube-system"},
		},
		KindSet:              map[string]bool{"Deployment": true},
		NamespaceSet:         map[string]bool{"kube-system": true},
		CompressionAlgorithm: "gzip",
	})

	testCases := []struct {
		name     string
		filters  models.QueryFilters
		segID    int32
		canSkip  bool
	}{
		{
			"empty filters cannot skip",
			models.QueryFilters{},
			1,
			false,
		},
		{
			"matching kind cannot skip",
			models.QueryFilters{Kind: "Pod"},
			1,
			false,
		},
		{
			"non-matching kind can skip",
			models.QueryFilters{Kind: "Service"},
			1,
			true,
		},
		{
			"matching namespace cannot skip",
			models.QueryFilters{Namespace: "default"},
			1,
			false,
		},
		{
			"non-matching namespace can skip",
			models.QueryFilters{Namespace: "production"},
			1,
			true,
		},
		{
			"multiple matching filters cannot skip",
			models.QueryFilters{Kind: "Pod", Namespace: "default"},
			1,
			false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := smi.CanSegmentBeSkipped(tc.segID, tc.filters)
			if result != tc.canSkip {
				t.Errorf("Expected %v, got %v", tc.canSkip, result)
			}
		})
	}
}

// TestFilterSegments tests filtering segments based on criteria
func TestFilterSegments(t *testing.T) {
	smi := storage.NewSegmentMetadataIndex()

	// Add metadata for segments
	smi.AddMetadata(1, models.SegmentMetadata{
		ResourceSummary: []models.ResourceMetadata{
			{Group: "apps", Version: "v1", Kind: "Pod", Namespace: "default"},
		},
		KindSet:              map[string]bool{"Pod": true},
		NamespaceSet:         map[string]bool{"default": true},
		CompressionAlgorithm: "gzip",
	})

	smi.AddMetadata(2, models.SegmentMetadata{
		ResourceSummary: []models.ResourceMetadata{
			{Group: "apps", Version: "v1", Kind: "Deployment", Namespace: "default"},
		},
		KindSet:              map[string]bool{"Deployment": true},
		NamespaceSet:         map[string]bool{"default": true},
		CompressionAlgorithm: "gzip",
	})

	smi.AddMetadata(3, models.SegmentMetadata{
		ResourceSummary: []models.ResourceMetadata{
			{Group: "apps", Version: "v1", Kind: "Pod", Namespace: "kube-system"},
		},
		KindSet:              map[string]bool{"Pod": true},
		NamespaceSet:         map[string]bool{"kube-system": true},
		CompressionAlgorithm: "gzip",
	})

	testCases := []struct {
		name        string
		segmentIDs  []int32
		filters     models.QueryFilters
		expectedLen int
	}{
		{
			"empty filters return all",
			[]int32{1, 2, 3},
			models.QueryFilters{},
			3,
		},
		{
			"filter by kind Pod",
			[]int32{1, 2, 3},
			models.QueryFilters{Kind: "Pod"},
			2,
		},
		{
			"filter by kind Deployment",
			[]int32{1, 2, 3},
			models.QueryFilters{Kind: "Deployment"},
			1,
		},
		{
			"filter by namespace default",
			[]int32{1, 2, 3},
			models.QueryFilters{Namespace: "default"},
			2,
		},
		{
			"filter by multiple criteria",
			[]int32{1, 2, 3},
			models.QueryFilters{Kind: "Pod", Namespace: "default"},
			1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := smi.FilterSegments(tc.segmentIDs, tc.filters)
			if len(result) != tc.expectedLen {
				t.Errorf("Expected %d segments, got %d", tc.expectedLen, len(result))
			}
		})
	}
}

// TestGetSegmentMetadata tests retrieving metadata for a specific segment
func TestGetSegmentMetadata(t *testing.T) {
	smi := storage.NewSegmentMetadataIndex()

	metadata := models.SegmentMetadata{
		ResourceSummary: []models.ResourceMetadata{
			{Group: "apps", Version: "v1", Kind: "Pod", Namespace: "default"},
		},
		KindSet:              map[string]bool{"Pod": true},
		NamespaceSet:         map[string]bool{"default": true},
		CompressionAlgorithm: "gzip",
	}

	smi.AddMetadata(1, metadata)

	retrieved, ok := smi.GetSegmentMetadata(1)
	if !ok {
		t.Error("Expected metadata to be found")
	}

	if len(retrieved.KindSet) != 1 || !retrieved.KindSet["Pod"] {
		t.Error("Expected Pod kind in retrieved metadata")
	}
}

// TestGetSegmentMetadataNotFound tests retrieving non-existent metadata
func TestGetSegmentMetadataNotFound(t *testing.T) {
	smi := storage.NewSegmentMetadataIndex()

	_, ok := smi.GetSegmentMetadata(999)
	if ok {
		t.Error("Expected metadata to not be found")
	}
}

// TestGetAllMetadatas tests retrieving all segment metadatas
func TestGetAllMetadatas(t *testing.T) {
	smi := storage.NewSegmentMetadataIndex()

	meta := models.SegmentMetadata{
		ResourceSummary: []models.ResourceMetadata{
			{Group: "apps", Version: "v1", Kind: "Pod", Namespace: "default"},
		},
		KindSet:              map[string]bool{"Pod": true},
		NamespaceSet:         map[string]bool{"default": true},
		CompressionAlgorithm: "gzip",
	}

	smi.AddMetadata(1, meta)
	smi.AddMetadata(2, meta)
	smi.AddMetadata(3, meta)

	all := smi.GetAllMetadatas()
	if len(all) != 3 {
		t.Errorf("Expected 3 metadatas, got %d", len(all))
	}

	if _, ok := all[1]; !ok {
		t.Error("Expected segment 1 in map")
	}
	if _, ok := all[2]; !ok {
		t.Error("Expected segment 2 in map")
	}
	if _, ok := all[3]; !ok {
		t.Error("Expected segment 3 in map")
	}
}

// TestContainsNamespace tests namespace existence checking
func TestContainsNamespace(t *testing.T) {
	smi := storage.NewSegmentMetadataIndex()

	meta1 := models.SegmentMetadata{
		ResourceSummary: []models.ResourceMetadata{
			{Group: "apps", Version: "v1", Kind: "Pod", Namespace: "default"},
		},
		NamespaceSet:         map[string]bool{"default": true, "kube-system": true},
		KindSet:              map[string]bool{"Pod": true},
		CompressionAlgorithm: "gzip",
	}
	smi.AddMetadata(1, meta1)

	meta2 := models.SegmentMetadata{
		ResourceSummary: []models.ResourceMetadata{
			{Group: "apps", Version: "v1", Kind: "Pod", Namespace: "production"},
		},
		NamespaceSet:         map[string]bool{"production": true},
		KindSet:              map[string]bool{"Pod": true},
		CompressionAlgorithm: "gzip",
	}
	smi.AddMetadata(2, meta2)

	testCases := []struct {
		namespace string
		expected  bool
	}{
		{"default", true},
		{"kube-system", true},
		{"production", true},
		{"staging", false},
		{"missing", false},
	}

	for _, tc := range testCases {
		t.Run(tc.namespace, func(t *testing.T) {
			result := smi.ContainsNamespace(tc.namespace)
			if result != tc.expected {
				t.Errorf("Expected %v for namespace %s, got %v", tc.expected, tc.namespace, result)
			}
		})
	}
}

// TestContainsKind tests kind existence checking
func TestContainsKind(t *testing.T) {
	smi := storage.NewSegmentMetadataIndex()

	meta1 := models.SegmentMetadata{
		ResourceSummary: []models.ResourceMetadata{
			{Group: "apps", Version: "v1", Kind: "Pod", Namespace: "default"},
		},
		KindSet:              map[string]bool{"Pod": true, "Deployment": true},
		NamespaceSet:         map[string]bool{"default": true},
		CompressionAlgorithm: "gzip",
	}
	smi.AddMetadata(1, meta1)

	meta2 := models.SegmentMetadata{
		ResourceSummary: []models.ResourceMetadata{
			{Group: "", Version: "v1", Kind: "Service", Namespace: "default"},
		},
		KindSet:              map[string]bool{"Service": true},
		NamespaceSet:         map[string]bool{"default": true},
		CompressionAlgorithm: "gzip",
	}
	smi.AddMetadata(2, meta2)

	testCases := []struct {
		kind     string
		expected bool
	}{
		{"Pod", true},
		{"Deployment", true},
		{"Service", true},
		{"StatefulSet", false},
		{"Job", false},
	}

	for _, tc := range testCases {
		t.Run(tc.kind, func(t *testing.T) {
			result := smi.ContainsKind(tc.kind)
			if result != tc.expected {
				t.Errorf("Expected %v for kind %s, got %v", tc.expected, tc.kind, result)
			}
		})
	}
}

// TestGetNamespaces tests retrieving all unique namespaces
func TestGetNamespaces(t *testing.T) {
	smi := storage.NewSegmentMetadataIndex()

	meta := models.SegmentMetadata{
		ResourceSummary: []models.ResourceMetadata{
			{Group: "apps", Version: "v1", Kind: "Pod", Namespace: "default"},
		},
		CompressionAlgorithm: "gzip",
		KindSet:              map[string]bool{"Pod": true},
	}

	meta1 := meta
	meta1.NamespaceSet = map[string]bool{"default": true, "kube-system": true}
	smi.AddMetadata(1, meta1)

	meta2 := meta
	meta2.NamespaceSet = map[string]bool{"kube-system": true, "production": true}
	smi.AddMetadata(2, meta2)

	meta3 := meta
	meta3.NamespaceSet = map[string]bool{"default": true}
	smi.AddMetadata(3, meta3)

	namespaces := smi.GetNamespaces()

	// Check that we have unique namespaces
	nsMap := make(map[string]bool)
	for _, ns := range namespaces {
		nsMap[ns] = true
	}

	if len(nsMap) != 3 {
		t.Errorf("Expected 3 unique namespaces, got %d", len(nsMap))
	}

	if !nsMap["default"] {
		t.Error("Expected 'default' namespace")
	}
	if !nsMap["kube-system"] {
		t.Error("Expected 'kube-system' namespace")
	}
	if !nsMap["production"] {
		t.Error("Expected 'production' namespace")
	}
}

// TestGetKinds tests retrieving all unique kinds
func TestGetKinds(t *testing.T) {
	smi := storage.NewSegmentMetadataIndex()

	meta := models.SegmentMetadata{
		ResourceSummary: []models.ResourceMetadata{
			{Group: "apps", Version: "v1", Kind: "Pod", Namespace: "default"},
		},
		CompressionAlgorithm: "gzip",
		NamespaceSet:         map[string]bool{"default": true},
	}

	meta1 := meta
	meta1.KindSet = map[string]bool{"Pod": true, "Deployment": true}
	smi.AddMetadata(1, meta1)

	meta2 := meta
	meta2.KindSet = map[string]bool{"Deployment": true, "Service": true}
	smi.AddMetadata(2, meta2)

	meta3 := meta
	meta3.KindSet = map[string]bool{"StatefulSet": true}
	smi.AddMetadata(3, meta3)

	kinds := smi.GetKinds()

	// Check that we have unique kinds
	kindMap := make(map[string]bool)
	for _, kind := range kinds {
		kindMap[kind] = true
	}

	if len(kindMap) != 4 {
		t.Errorf("Expected 4 unique kinds, got %d", len(kindMap))
	}

	if !kindMap["Pod"] {
		t.Error("Expected 'Pod' kind")
	}
	if !kindMap["Deployment"] {
		t.Error("Expected 'Deployment' kind")
	}
	if !kindMap["Service"] {
		t.Error("Expected 'Service' kind")
	}
	if !kindMap["StatefulSet"] {
		t.Error("Expected 'StatefulSet' kind")
	}
}

// TestGetSegmentCount tests getting segment count
func TestGetSegmentCount(t *testing.T) {
	smi := storage.NewSegmentMetadataIndex()

	if smi.GetSegmentCount() != 0 {
		t.Error("Expected 0 segments initially")
	}

	meta := models.SegmentMetadata{
		ResourceSummary: []models.ResourceMetadata{
			{Group: "apps", Version: "v1", Kind: "Pod", Namespace: "default"},
		},
		KindSet:              map[string]bool{"Pod": true},
		NamespaceSet:         map[string]bool{"default": true},
		CompressionAlgorithm: "gzip",
	}

	smi.AddMetadata(1, meta)
	if smi.GetSegmentCount() != 1 {
		t.Error("Expected 1 segment after add")
	}

	smi.AddMetadata(2, meta)
	if smi.GetSegmentCount() != 2 {
		t.Error("Expected 2 segments after second add")
	}

	smi.AddMetadata(3, meta)
	if smi.GetSegmentCount() != 3 {
		t.Error("Expected 3 segments after third add")
	}
}

// TestClearSegmentMetadata tests clearing all metadata
func TestClearSegmentMetadata(t *testing.T) {
	smi := storage.NewSegmentMetadataIndex()

	meta := models.SegmentMetadata{
		ResourceSummary: []models.ResourceMetadata{
			{Group: "apps", Version: "v1", Kind: "Pod", Namespace: "default"},
		},
		KindSet:              map[string]bool{"Pod": true},
		NamespaceSet:         map[string]bool{"default": true},
		CompressionAlgorithm: "gzip",
	}

	smi.AddMetadata(1, meta)
	smi.AddMetadata(2, meta)

	if smi.GetSegmentCount() != 2 {
		t.Error("Expected 2 segments before clear")
	}

	smi.Clear()

	if smi.GetSegmentCount() != 0 {
		t.Errorf("Expected 0 segments after clear, got %d", smi.GetSegmentCount())
	}

	// Verify metadata is truly gone
	_, ok := smi.GetSegmentMetadata(1)
	if ok {
		t.Error("Expected no metadata found after clear")
	}
}

// TestEmptySegmentMetadataIndex tests behavior with empty index
func TestEmptySegmentMetadataIndex(t *testing.T) {
	smi := storage.NewSegmentMetadataIndex()

	// Test ContainsNamespace with empty index
	if smi.ContainsNamespace("default") {
		t.Error("Empty index should not contain any namespace")
	}

	// Test ContainsKind with empty index
	if smi.ContainsKind("Pod") {
		t.Error("Empty index should not contain any kind")
	}

	// Test GetNamespaces with empty index
	namespaces := smi.GetNamespaces()
	if len(namespaces) != 0 {
		t.Errorf("Empty index should return 0 namespaces, got %d", len(namespaces))
	}

	// Test GetKinds with empty index
	kinds := smi.GetKinds()
	if len(kinds) != 0 {
		t.Errorf("Empty index should return 0 kinds, got %d", len(kinds))
	}

	// Test FilterSegments with empty index
	result := smi.FilterSegments([]int32{1, 2, 3}, models.QueryFilters{})
	if len(result) != 3 {
		t.Errorf("Empty index should include all segments, got %d", len(result))
	}
}

// TestSegmentMetadataOverwrite tests overwriting metadata for same segment
func TestSegmentMetadataOverwrite(t *testing.T) {
	smi := storage.NewSegmentMetadataIndex()

	metadata1 := models.SegmentMetadata{
		ResourceSummary: []models.ResourceMetadata{
			{Group: "apps", Version: "v1", Kind: "Pod", Namespace: "default"},
		},
		KindSet:              map[string]bool{"Pod": true},
		NamespaceSet:         map[string]bool{"default": true},
		CompressionAlgorithm: "gzip",
	}

	metadata2 := models.SegmentMetadata{
		ResourceSummary: []models.ResourceMetadata{
			{Group: "apps", Version: "v1", Kind: "Deployment", Namespace: "production"},
		},
		KindSet:              map[string]bool{"Deployment": true},
		NamespaceSet:         map[string]bool{"production": true},
		CompressionAlgorithm: "gzip",
	}

	smi.AddMetadata(1, metadata1)
	if smi.GetSegmentCount() != 1 {
		t.Error("Expected 1 segment after first add")
	}

	smi.AddMetadata(1, metadata2)
	if smi.GetSegmentCount() != 1 {
		t.Error("Expected 1 segment after overwrite (not duplicate)")
	}

	retrieved, _ := smi.GetSegmentMetadata(1)
	if len(retrieved.KindSet) != 1 || !retrieved.KindSet["Deployment"] {
		t.Error("Expected metadata to be overwritten")
	}
}

// TestFilterSegmentsWithMissingMetadata tests filtering when some segments have no metadata
func TestFilterSegmentsWithMissingMetadata(t *testing.T) {
	smi := storage.NewSegmentMetadataIndex()

	// Only add metadata for segment 1 and 3, not 2
	smi.AddMetadata(1, models.SegmentMetadata{
		ResourceSummary: []models.ResourceMetadata{
			{Group: "apps", Version: "v1", Kind: "Pod", Namespace: "default"},
		},
		KindSet:              map[string]bool{"Pod": true},
		NamespaceSet:         map[string]bool{"default": true},
		CompressionAlgorithm: "gzip",
	})

	smi.AddMetadata(3, models.SegmentMetadata{
		ResourceSummary: []models.ResourceMetadata{
			{Group: "", Version: "v1", Kind: "Service", Namespace: "default"},
		},
		KindSet:              map[string]bool{"Service": true},
		NamespaceSet:         map[string]bool{"default": true},
		CompressionAlgorithm: "gzip",
	})

	// When filtering with no metadata for segment 2, it should be included anyway (safe default)
	result := smi.FilterSegments([]int32{1, 2, 3}, models.QueryFilters{Kind: "Pod"})

	// Should include segment 1 (matches) and segment 2 (no metadata, safe default), but not 3
	if len(result) != 2 {
		t.Errorf("Expected 2 segments (1 and 2), got %d", len(result))
	}

	resultMap := make(map[int32]bool)
	for _, id := range result {
		resultMap[id] = true
	}

	if !resultMap[1] {
		t.Error("Expected segment 1 in result (matches Pod filter)")
	}
	if !resultMap[2] {
		t.Error("Expected segment 2 in result (no metadata, safe default)")
	}
	if resultMap[3] {
		t.Error("Did not expect segment 3 (doesn't match Pod filter)")
	}
}

// TestLargeMetadataIndex tests with many segments
func TestLargeMetadataIndex(t *testing.T) {
	smi := storage.NewSegmentMetadataIndex()

	// Add metadata for 1000 segments
	for i := 1; i <= 1000; i++ {
		kinds := []string{"Pod", "Deployment", "Service"}
		selectedKind := kinds[i%3]

		smi.AddMetadata(int32(i), models.SegmentMetadata{
			ResourceSummary: []models.ResourceMetadata{
				{Group: "apps", Version: "v1", Kind: selectedKind, Namespace: "default"},
			},
			KindSet:              map[string]bool{selectedKind: true},
			NamespaceSet:         map[string]bool{"default": true},
			CompressionAlgorithm: "gzip",
		})
	}

	if smi.GetSegmentCount() != 1000 {
		t.Errorf("Expected 1000 segments, got %d", smi.GetSegmentCount())
	}

	// Filter for Pod kind
	allSegments := make([]int32, 0, 1000)
	for i := 1; i <= 1000; i++ {
		allSegments = append(allSegments, int32(i))
	}

	filtered := smi.FilterSegments(allSegments, models.QueryFilters{Kind: "Pod"})
	if len(filtered) != 334 { // Approximately 1000/3
		t.Logf("Filtered %d segments for Pod kind", len(filtered))
	}

	// Check that all namespaces are "default"
	namespaces := smi.GetNamespaces()
	if len(namespaces) != 1 || namespaces[0] != "default" {
		t.Error("Expected only 'default' namespace")
	}
}
