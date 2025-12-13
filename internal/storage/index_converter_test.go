package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertToProtoAndBack(t *testing.T) {
	// Create sample IndexSection
	original := &IndexSection{
		FormatVersion: "2.0",
		BlockMetadata: []*BlockMetadata{
			{
				ID:                 0,
				Offset:             1024,
				CompressedLength:   512,
				UncompressedLength: 2048,
				EventCount:         10,
				TimestampMin:       1000000,
				TimestampMax:       2000000,
				Checksum:           "abc123",
				KindSet:            []string{"Pod", "Deployment"},
				NamespaceSet:       []string{"default", "kube-system"},
				GroupSet:           []string{"apps"},
			},
		},
		InvertedIndexes: &InvertedIndex{
			KindToBlocks: map[string][]int32{
				"Pod": {0, 1, 2},
			},
			NamespaceToBlocks: map[string][]int32{
				"default": {0, 1},
			},
			GroupToBlocks: map[string][]int32{
				"apps": {1, 2},
			},
		},
		Statistics: &IndexStatistics{
			TotalBlocks:            3,
			TotalEvents:            100,
			TotalUncompressedBytes: 10240,
			TotalCompressedBytes:   5120,
			CompressionRatio:       2.0,
			UniqueKinds:            5,
			UniqueNamespaces:       3,
			UniqueGroups:           2,
			TimestampMin:           1000000,
			TimestampMax:           2000000,
		},
		FinalResourceStates: map[string]*ResourceLastState{
			"test-key": {
				UID:          "test-uid",
				EventType:    "CREATE",
				Timestamp:    1500000,
				ResourceData: []byte(`{"kind":"Pod"}`),
			},
		},
	}

	// Convert to protobuf and back
	pbSection := convertToProto(original)
	require.NotNil(t, pbSection)

	reconstructed := convertFromProto(pbSection)
	require.NotNil(t, reconstructed)

	// Verify all fields match
	assert.Equal(t, original.FormatVersion, reconstructed.FormatVersion)
	assert.Equal(t, len(original.BlockMetadata), len(reconstructed.BlockMetadata))

	// Verify block metadata
	for i, origBM := range original.BlockMetadata {
		reconBM := reconstructed.BlockMetadata[i]
		assert.Equal(t, origBM.ID, reconBM.ID)
		assert.Equal(t, origBM.Offset, reconBM.Offset)
		assert.Equal(t, origBM.CompressedLength, reconBM.CompressedLength)
		assert.Equal(t, origBM.UncompressedLength, reconBM.UncompressedLength)
		assert.Equal(t, origBM.EventCount, reconBM.EventCount)
		assert.Equal(t, origBM.TimestampMin, reconBM.TimestampMin)
		assert.Equal(t, origBM.TimestampMax, reconBM.TimestampMax)
		assert.Equal(t, origBM.Checksum, reconBM.Checksum)
		assert.Equal(t, origBM.KindSet, reconBM.KindSet)
		assert.Equal(t, origBM.NamespaceSet, reconBM.NamespaceSet)
		assert.Equal(t, origBM.GroupSet, reconBM.GroupSet)
	}

	// Verify inverted indexes
	assert.Equal(t, len(original.InvertedIndexes.KindToBlocks), len(reconstructed.InvertedIndexes.KindToBlocks))
	assert.Equal(t, original.InvertedIndexes.KindToBlocks["Pod"], reconstructed.InvertedIndexes.KindToBlocks["Pod"])

	// Verify statistics
	assert.Equal(t, original.Statistics.TotalBlocks, reconstructed.Statistics.TotalBlocks)
	assert.Equal(t, original.Statistics.TotalEvents, reconstructed.Statistics.TotalEvents)
	assert.Equal(t, original.Statistics.CompressionRatio, reconstructed.Statistics.CompressionRatio)

	// Verify final resource states
	assert.Equal(t, len(original.FinalResourceStates), len(reconstructed.FinalResourceStates))
	origState := original.FinalResourceStates["test-key"]
	reconState := reconstructed.FinalResourceStates["test-key"]
	assert.Equal(t, origState.UID, reconState.UID)
	assert.Equal(t, origState.EventType, reconState.EventType)
	assert.Equal(t, origState.Timestamp, reconState.Timestamp)
	assert.Equal(t, string(origState.ResourceData), string(reconState.ResourceData))
}

func TestConvertBloomFilterToProtoAndBack(t *testing.T) {
	// Create a bloom filter
	bf := NewBloomFilter(100, 0.01)
	bf.Add("Pod")
	bf.Add("Deployment")
	bf.Add("Service")

	// Convert to protobuf and back
	pbBf := convertBloomFilterToProto(bf)
	require.NotNil(t, pbBf)

	reconstructed := convertBloomFilterFromProto(pbBf)
	require.NotNil(t, reconstructed)

	// Verify the bloom filter works
	assert.True(t, reconstructed.Contains("Pod"))
	assert.True(t, reconstructed.Contains("Deployment"))
	assert.True(t, reconstructed.Contains("Service"))
	assert.False(t, reconstructed.Contains("NonExistent"))
}
