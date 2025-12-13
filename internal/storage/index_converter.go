package storage

import (
	"encoding/binary"
	"encoding/json"

	"github.com/bits-and-blooms/bitset"
	"github.com/bits-and-blooms/bloom/v3"
)

// convertToProto converts IndexSection to protobuf message
func convertToProto(section *IndexSection) *PBIndexSection {
	pbSection := &PBIndexSection{
		FormatVersion: section.FormatVersion,
		BlockMetadata: make([]*PBBlockMetadata, len(section.BlockMetadata)),
	}

	// Convert BlockMetadata
	for i, bm := range section.BlockMetadata {
		pbSection.BlockMetadata[i] = &PBBlockMetadata{
			Id:                 bm.ID,
			Offset:             bm.Offset,
			CompressedLength:   bm.CompressedLength,
			UncompressedLength: bm.UncompressedLength,
			EventCount:         bm.EventCount,
			TimestampMin:       bm.TimestampMin,
			TimestampMax:       bm.TimestampMax,
			Checksum:           bm.Checksum,
			KindSet:            bm.KindSet,
			NamespaceSet:       bm.NamespaceSet,
			GroupSet:           bm.GroupSet,
		}

		// Convert bloom filters
		if bm.BloomFilterKinds != nil {
			pbSection.BlockMetadata[i].BloomFilterKinds = convertBloomFilterToProto(bm.BloomFilterKinds)
		}
		if bm.BloomFilterNamespaces != nil {
			pbSection.BlockMetadata[i].BloomFilterNamespaces = convertBloomFilterToProto(bm.BloomFilterNamespaces)
		}
		if bm.BloomFilterGroups != nil {
			pbSection.BlockMetadata[i].BloomFilterGroups = convertBloomFilterToProto(bm.BloomFilterGroups)
		}
	}

	// Convert InvertedIndex
	if section.InvertedIndexes != nil {
		pbSection.InvertedIndexes = convertInvertedIndexToProto(section.InvertedIndexes)
	}

	// Convert Statistics
	if section.Statistics != nil {
		pbSection.Statistics = convertStatisticsToProto(section.Statistics)
	}

	// Convert FinalResourceStates
	if len(section.FinalResourceStates) > 0 {
		pbSection.FinalResourceStates = make(map[string]*PBResourceLastState)
		for key, state := range section.FinalResourceStates {
			pbSection.FinalResourceStates[key] = convertResourceStateToProto(state)
		}
	}

	return pbSection
}

// convertFromProto converts protobuf message to IndexSection
func convertFromProto(pbSection *PBIndexSection) *IndexSection {
	section := &IndexSection{
		FormatVersion: pbSection.FormatVersion,
		BlockMetadata: make([]*BlockMetadata, len(pbSection.BlockMetadata)),
	}

	// Convert BlockMetadata
	for i, pbBm := range pbSection.BlockMetadata {
		section.BlockMetadata[i] = &BlockMetadata{
			ID:                 pbBm.Id,
			Offset:             pbBm.Offset,
			CompressedLength:   pbBm.CompressedLength,
			UncompressedLength: pbBm.UncompressedLength,
			EventCount:         pbBm.EventCount,
			TimestampMin:       pbBm.TimestampMin,
			TimestampMax:       pbBm.TimestampMax,
			Checksum:           pbBm.Checksum,
			KindSet:            pbBm.KindSet,
			NamespaceSet:       pbBm.NamespaceSet,
			GroupSet:           pbBm.GroupSet,
		}

		// Convert bloom filters
		if pbBm.BloomFilterKinds != nil {
			section.BlockMetadata[i].BloomFilterKinds = convertBloomFilterFromProto(pbBm.BloomFilterKinds)
		}
		if pbBm.BloomFilterNamespaces != nil {
			section.BlockMetadata[i].BloomFilterNamespaces = convertBloomFilterFromProto(pbBm.BloomFilterNamespaces)
		}
		if pbBm.BloomFilterGroups != nil {
			section.BlockMetadata[i].BloomFilterGroups = convertBloomFilterFromProto(pbBm.BloomFilterGroups)
		}
	}

	// Convert InvertedIndex
	if pbSection.InvertedIndexes != nil {
		section.InvertedIndexes = convertInvertedIndexFromProto(pbSection.InvertedIndexes)
	}

	// Convert Statistics
	if pbSection.Statistics != nil {
		section.Statistics = convertStatisticsFromProto(pbSection.Statistics)
	}

	// Convert FinalResourceStates
	if len(pbSection.FinalResourceStates) > 0 {
		section.FinalResourceStates = make(map[string]*ResourceLastState)
		for key, pbState := range pbSection.FinalResourceStates {
			section.FinalResourceStates[key] = convertResourceStateFromProto(pbState)
		}
	}

	return section
}

// convertBloomFilterToProto converts StandardBloomFilter to protobuf
func convertBloomFilterToProto(bf *StandardBloomFilter) *PBBloomFilter {
	if bf == nil || bf.filter == nil {
		return nil
	}

	// Get the bitset as uint64 array
	bits := bf.filter.BitSet().Bytes()

	// Convert uint64 array to bytes for protobuf
	bitsetBytes := make([]byte, len(bits)*8)
	for i, v := range bits {
		binary.LittleEndian.PutUint64(bitsetBytes[i*8:], v)
	}

	return &PBBloomFilter{
		Size:      uint32(bf.filter.BitSet().Len()),
		NumHashes: uint32(bf.filter.K()),
		Bitset:    bitsetBytes,
	}
}

// convertBloomFilterFromProto converts protobuf BloomFilter to StandardBloomFilter
func convertBloomFilterFromProto(pbBf *PBBloomFilter) *StandardBloomFilter {
	if pbBf == nil {
		return nil
	}

	// Convert bytes back to uint64 array
	numUint64s := len(pbBf.Bitset) / 8
	bits := make([]uint64, numUint64s)
	for i := 0; i < numUint64s; i++ {
		bits[i] = binary.LittleEndian.Uint64(pbBf.Bitset[i*8:])
	}

	// Create bloom filter from the data
	bloomFilter := bloom.FromWithM(bits, uint(pbBf.Size), uint(pbBf.NumHashes))

	return &StandardBloomFilter{
		filter:        bloomFilter,
		hashFunctions: uint(pbBf.NumHashes),
	}
}

// convertInvertedIndexToProto converts InvertedIndex to protobuf
func convertInvertedIndexToProto(idx *InvertedIndex) *PBInvertedIndex {
	if idx == nil {
		return nil
	}

	pbIdx := &PBInvertedIndex{
		KindToBlocks:      make(map[string]*PBBlockIDList),
		NamespaceToBlocks: make(map[string]*PBBlockIDList),
		GroupToBlocks:     make(map[string]*PBBlockIDList),
	}

	// Convert KindToBlocks
	for key, blockIDs := range idx.KindToBlocks {
		pbIdx.KindToBlocks[key] = &PBBlockIDList{BlockIds: blockIDs}
	}

	// Convert NamespaceToBlocks
	for key, blockIDs := range idx.NamespaceToBlocks {
		pbIdx.NamespaceToBlocks[key] = &PBBlockIDList{BlockIds: blockIDs}
	}

	// Convert GroupToBlocks
	for key, blockIDs := range idx.GroupToBlocks {
		pbIdx.GroupToBlocks[key] = &PBBlockIDList{BlockIds: blockIDs}
	}

	return pbIdx
}

// convertInvertedIndexFromProto converts protobuf InvertedIndex to InvertedIndex
func convertInvertedIndexFromProto(pbIdx *PBInvertedIndex) *InvertedIndex {
	if pbIdx == nil {
		return nil
	}

	idx := &InvertedIndex{
		KindToBlocks:      make(map[string][]int32),
		NamespaceToBlocks: make(map[string][]int32),
		GroupToBlocks:     make(map[string][]int32),
	}

	// Convert KindToBlocks
	for key, blockIDList := range pbIdx.KindToBlocks {
		idx.KindToBlocks[key] = blockIDList.BlockIds
	}

	// Convert NamespaceToBlocks
	for key, blockIDList := range pbIdx.NamespaceToBlocks {
		idx.NamespaceToBlocks[key] = blockIDList.BlockIds
	}

	// Convert GroupToBlocks
	for key, blockIDList := range pbIdx.GroupToBlocks {
		idx.GroupToBlocks[key] = blockIDList.BlockIds
	}

	return idx
}

// convertStatisticsToProto converts IndexStatistics to protobuf
func convertStatisticsToProto(stats *IndexStatistics) *PBIndexStatistics {
	if stats == nil {
		return nil
	}

	return &PBIndexStatistics{
		TotalBlocks:            stats.TotalBlocks,
		TotalEvents:            stats.TotalEvents,
		TotalUncompressedBytes: stats.TotalUncompressedBytes,
		TotalCompressedBytes:   stats.TotalCompressedBytes,
		CompressionRatio:       stats.CompressionRatio,
		UniqueKinds:            stats.UniqueKinds,
		UniqueNamespaces:       stats.UniqueNamespaces,
		UniqueGroups:           stats.UniqueGroups,
		TimestampMin:           stats.TimestampMin,
		TimestampMax:           stats.TimestampMax,
	}
}

// convertStatisticsFromProto converts protobuf IndexStatistics to IndexStatistics
func convertStatisticsFromProto(pbStats *PBIndexStatistics) *IndexStatistics {
	if pbStats == nil {
		return nil
	}

	return &IndexStatistics{
		TotalBlocks:            pbStats.TotalBlocks,
		TotalEvents:            pbStats.TotalEvents,
		TotalUncompressedBytes: pbStats.TotalUncompressedBytes,
		TotalCompressedBytes:   pbStats.TotalCompressedBytes,
		CompressionRatio:       pbStats.CompressionRatio,
		UniqueKinds:            pbStats.UniqueKinds,
		UniqueNamespaces:       pbStats.UniqueNamespaces,
		UniqueGroups:           pbStats.UniqueGroups,
		TimestampMin:           pbStats.TimestampMin,
		TimestampMax:           pbStats.TimestampMax,
	}
}

// convertResourceStateToProto converts ResourceLastState to protobuf
func convertResourceStateToProto(state *ResourceLastState) *PBResourceLastState {
	if state == nil {
		return nil
	}

	return &PBResourceLastState{
		Uid:          state.UID,
		EventType:    state.EventType,
		Timestamp:    state.Timestamp,
		ResourceData: []byte(state.ResourceData),
	}
}

// convertResourceStateFromProto converts protobuf ResourceLastState to ResourceLastState
func convertResourceStateFromProto(pbState *PBResourceLastState) *ResourceLastState {
	if pbState == nil {
		return nil
	}

	return &ResourceLastState{
		UID:          pbState.Uid,
		EventType:    pbState.EventType,
		Timestamp:    pbState.Timestamp,
		ResourceData: json.RawMessage(pbState.ResourceData),
	}
}

// NewBitsetFromBytes creates a bitset from raw bytes
func NewBitsetFromBytes(data []byte) *bitset.BitSet {
	numUint64s := len(data) / 8
	bits := make([]uint64, numUint64s)
	for i := 0; i < numUint64s; i++ {
		bits[i] = binary.LittleEndian.Uint64(data[i*8:])
	}
	return bitset.From(bits)
}
