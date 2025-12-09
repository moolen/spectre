package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/moolen/spectre/internal/models"
	"github.com/moolen/spectre/internal/storage"
	"github.com/spf13/cobra"
)

var debugCmd = &cobra.Command{
	Use:   "debug",
	Short: "Debug utilities for Spectre",
	Long:  `Various debugging and inspection tools for Spectre internals.`,
}

var (
	debugStorageFile      string
	debugStorageGroup     string
	debugStorageVersion   string
	debugStorageKind      string
	debugStorageNamespace string
	debugStorageName      string
	debugStorageExtended  bool
	debugStorageJSON      bool
)

var debugStorageCmd = &cobra.Command{
	Use:   "storage",
	Short: "Inspect Spectre storage files",
	Long: `Debug utility to inspect and analyze Spectre's binary storage files.
Shows file headers, statistics, blocks, and events with optional filtering.`,
	Run: runDebugStorage,
}

func init() {
	debugStorageCmd.Flags().StringVar(&debugStorageFile, "file", "", "Path to storage file (required)")
	debugStorageCmd.Flags().StringVar(&debugStorageGroup, "group", "", "Filter by API group")
	debugStorageCmd.Flags().StringVar(&debugStorageVersion, "version", "", "Filter by API version")
	debugStorageCmd.Flags().StringVar(&debugStorageKind, "kind", "", "Filter by resource kind")
	debugStorageCmd.Flags().StringVar(&debugStorageNamespace, "namespace", "", "Filter by namespace")
	debugStorageCmd.Flags().StringVar(&debugStorageName, "name", "", "Filter by resource name")
	debugStorageCmd.Flags().BoolVarP(&debugStorageExtended, "extended", "x", false, "Show extended details")
	debugStorageCmd.Flags().BoolVar(&debugStorageJSON, "json", false, "Output as JSON")

	debugStorageCmd.MarkFlagRequired("file")

	debugCmd.AddCommand(debugStorageCmd)
}

func runDebugStorage(cmd *cobra.Command, args []string) {
	// Read storage file
	data, err := readDebugStorageFile(debugStorageFile)
	if err != nil {
		HandleError(err, "Failed to read storage file")
	}

	// Apply filters
	filter := &debugEventFilter{
		Group:     debugStorageGroup,
		Version:   debugStorageVersion,
		Kind:      debugStorageKind,
		Namespace: debugStorageNamespace,
		Name:      debugStorageName,
	}

	filteredEvents := applyDebugFilter(data.Events, filter)

	// Format and print output
	if debugStorageJSON {
		printDebugJSON(data, filteredEvents)
	} else if debugStorageExtended {
		printDebugExtended(data, filteredEvents, filter)
	} else {
		printDebugSummary(data, filteredEvents, filter)
	}
}

// Debug storage data structures

type debugFileData struct {
	Header              *storage.FileHeader
	Footer              *storage.FileFooter
	IndexSection        *storage.IndexSection
	Blocks              []*debugBlockData
	Events              []*models.Event
	Statistics          map[string]interface{}
	FinalResourceStates map[string]*storage.ResourceLastState
}

type debugBlockData struct {
	Metadata *storage.BlockMetadata
	Events   []*models.Event
}

type debugEventFilter struct {
	Group     string
	Version   string
	Kind      string
	Namespace string
	Name      string
}

// readDebugStorageFile reads a complete storage file
func readDebugStorageFile(filePath string) (*debugFileData, error) {
	reader, err := storage.NewBlockReader(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create reader: %w", err)
	}
	defer reader.Close()

	header, err := reader.ReadFileHeader()
	if err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	footer, err := reader.ReadFileFooter()
	if err != nil {
		return nil, fmt.Errorf("failed to read footer: %w", err)
	}

	indexSection, err := reader.ReadIndexSection(footer.IndexSectionOffset, footer.IndexSectionLength)
	if err != nil {
		return nil, fmt.Errorf("failed to read index: %w", err)
	}

	var blocks []*debugBlockData
	var allEvents []*models.Event

	for i := 0; i < len(indexSection.BlockMetadata); i++ {
		metadata := indexSection.BlockMetadata[i]
		events, err := reader.ReadBlockEvents(metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to read block %d: %w", i, err)
		}

		blocks = append(blocks, &debugBlockData{
			Metadata: metadata,
			Events:   events,
		})
		allEvents = append(allEvents, events...)
	}

	stats := buildDebugStatistics(header, footer, indexSection, blocks, allEvents)

	return &debugFileData{
		Header:              header,
		Footer:              footer,
		IndexSection:        indexSection,
		Blocks:              blocks,
		Events:              allEvents,
		Statistics:          stats,
		FinalResourceStates: indexSection.FinalResourceStates,
	}, nil
}

// buildDebugStatistics calculates file statistics
func buildDebugStatistics(header *storage.FileHeader, footer *storage.FileFooter,
	index *storage.IndexSection, blocks []*debugBlockData, events []*models.Event) map[string]interface{} {

	stats := make(map[string]interface{})

	stats["format_version"] = header.FormatVersion
	stats["compression"] = header.CompressionAlgorithm
	stats["encoding"] = header.EncodingFormat
	stats["block_size"] = header.BlockSize
	stats["checksum_enabled"] = header.ChecksumEnabled
	stats["created_at"] = time.Unix(0, header.CreatedAt)
	stats["total_blocks"] = len(blocks)
	stats["total_events"] = len(events)

	var compressedSize, uncompressedSize uint64
	for _, block := range blocks {
		compressedSize += uint64(block.Metadata.CompressedLength)
		uncompressedSize += uint64(block.Metadata.UncompressedLength)
	}
	stats["compressed_size"] = compressedSize
	stats["uncompressed_size"] = uncompressedSize
	if uncompressedSize > 0 {
		stats["compression_ratio"] = float64(compressedSize) / float64(uncompressedSize)
	}

	kinds := make(map[string]bool)
	namespaces := make(map[string]bool)
	groups := make(map[string]bool)

	var minTime, maxTime int64
	for i, event := range events {
		kinds[event.Resource.Kind] = true
		if event.Resource.Namespace != "" {
			namespaces[event.Resource.Namespace] = true
		}
		groups[event.Resource.Group] = true

		if i == 0 || event.Timestamp < minTime {
			minTime = event.Timestamp
		}
		if i == 0 || event.Timestamp > maxTime {
			maxTime = event.Timestamp
		}
	}

	stats["unique_kinds"] = len(kinds)
	stats["unique_namespaces"] = len(namespaces)
	stats["unique_groups"] = len(groups)
	stats["min_timestamp"] = time.Unix(0, minTime)
	stats["max_timestamp"] = time.Unix(0, maxTime)

	return stats
}

// applyDebugFilter filters events based on criteria
func applyDebugFilter(events []*models.Event, filter *debugEventFilter) []*models.Event {
	if isDebugFilterEmpty(filter) {
		return events
	}

	var filtered []*models.Event
	for _, event := range events {
		if matchesDebugFilter(event, filter) {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

func matchesDebugFilter(event *models.Event, filter *debugEventFilter) bool {
	if filter.Group != "" && event.Resource.Group != filter.Group {
		return false
	}
	if filter.Version != "" && event.Resource.Version != filter.Version {
		return false
	}
	if filter.Kind != "" && event.Resource.Kind != filter.Kind {
		return false
	}
	if filter.Namespace != "" && event.Resource.Namespace != filter.Namespace {
		return false
	}
	if filter.Name != "" && event.Resource.Name != filter.Name {
		return false
	}
	return true
}

func isDebugFilterEmpty(filter *debugEventFilter) bool {
	return filter.Group == "" && filter.Version == "" && filter.Kind == "" &&
		filter.Namespace == "" && filter.Name == ""
}

// Output formatters

func printDebugSummary(data *debugFileData, events []*models.Event, filter *debugEventFilter) {
	fmt.Println("=== File Header ===")
	fmt.Printf("Format Version:    %s\n", data.Header.FormatVersion)
	fmt.Printf("Compression:       %s\n", data.Header.CompressionAlgorithm)
	fmt.Printf("Encoding:          %s\n", data.Header.EncodingFormat)
	fmt.Printf("Target Block Size: %s\n", formatDebugBytes(uint64(data.Header.BlockSize)))
	fmt.Printf("Checksum Enabled:  %v\n", data.Header.ChecksumEnabled)
	fmt.Printf("Created:           %s\n\n", time.Unix(0, data.Header.CreatedAt).Format(time.RFC3339))

	fmt.Println("=== File Statistics ===")
	fmt.Printf("Total Blocks:         %d\n", data.Statistics["total_blocks"])
	fmt.Printf("Total Events:         %d\n", data.Statistics["total_events"])
	fmt.Printf("Compressed Size:      %s\n", formatDebugBytes(data.Statistics["compressed_size"].(uint64)))
	fmt.Printf("Uncompressed Size:    %s\n", formatDebugBytes(data.Statistics["uncompressed_size"].(uint64)))
	if ratio, ok := data.Statistics["compression_ratio"].(float64); ok {
		fmt.Printf("Compression Ratio:    %.2f%%\n", ratio*100)
	}
	fmt.Printf("Unique Kinds:         %d\n", data.Statistics["unique_kinds"])
	fmt.Printf("Unique Namespaces:    %d\n", data.Statistics["unique_namespaces"])
	fmt.Printf("Unique Groups:        %d\n", data.Statistics["unique_groups"])
	if minTime, ok := data.Statistics["min_timestamp"].(time.Time); ok {
		fmt.Printf("Time Range:           %s to %s\n\n",
			minTime.Format(time.RFC3339),
			data.Statistics["max_timestamp"].(time.Time).Format(time.RFC3339))
	}

	fmt.Println("=== Index Section ===")
	fmt.Printf("Final Resource States: %d\n\n", len(data.FinalResourceStates))

	fmt.Println("=== Blocks ===")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Block ID\tOffset\tCompressed\tUncompressed\tEvents")
	for i, block := range data.Blocks {
		fmt.Fprintf(w, "%d\t%d\t%s\t%s\t%d\n",
			i,
			block.Metadata.Offset,
			formatDebugBytes(uint64(block.Metadata.CompressedLength)),
			formatDebugBytes(uint64(block.Metadata.UncompressedLength)),
			block.Metadata.EventCount)
	}
	w.Flush()

	if !isDebugFilterEmpty(filter) {
		fmt.Println("\n=== Filter Results ===")
		printDebugFilterInfo(filter)
		fmt.Printf("Matching Events: %d\n", len(events))

		uniqueResources := getDebugUniqueResources(events)
		fmt.Printf("Unique Resources: %d\n", len(uniqueResources))
	}
}

func printDebugExtended(data *debugFileData, events []*models.Event, filter *debugEventFilter) {
	printDebugSummary(data, events, filter)

	fmt.Println("\n=== Detailed Block Information ===")
	for i, block := range data.Blocks {
		fmt.Printf("\n--- Block %d ---\n", i)
		fmt.Printf("Offset:         %d\n", block.Metadata.Offset)
		fmt.Printf("Compressed:     %s\n", formatDebugBytes(uint64(block.Metadata.CompressedLength)))
		fmt.Printf("Uncompressed:   %s\n", formatDebugBytes(uint64(block.Metadata.UncompressedLength)))
		fmt.Printf("Event Count:    %d\n", block.Metadata.EventCount)
		fmt.Printf("Checksum:       %s\n", block.Metadata.Checksum)
		fmt.Printf("Min Timestamp:  %s\n", time.Unix(0, block.Metadata.TimestampMin).Format(time.RFC3339))
		fmt.Printf("Max Timestamp:  %s\n", time.Unix(0, block.Metadata.TimestampMax).Format(time.RFC3339))
	}

	fmt.Println("\n=== Events ===")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Timestamp\tType\tKind\tNamespace\tName\tSize")
	for _, event := range events {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			time.Unix(0, event.Timestamp).Format("15:04:05"),
			debugEventTypeString(event.Type),
			event.Resource.Kind,
			event.Resource.Namespace,
			event.Resource.Name,
			formatDebugBytes(uint64(len(event.Data))))
	}
	w.Flush()

	if len(data.FinalResourceStates) > 0 {
		fmt.Println("\n=== Final Resource States ===")
		printDebugFinalResourceStates(data.FinalResourceStates)
	}
}

func printDebugJSON(data *debugFileData, events []*models.Event) {
	output := map[string]interface{}{
		"header":     data.Header,
		"statistics": data.Statistics,
		"blocks":     len(data.Blocks),
		"events":     events,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		HandleError(err, "Failed to encode JSON")
	}
}

func printDebugFilterInfo(filter *debugEventFilter) {
	fmt.Println("Active Filters:")
	if filter.Group != "" {
		fmt.Printf("  Group:     %s\n", filter.Group)
	}
	if filter.Version != "" {
		fmt.Printf("  Version:   %s\n", filter.Version)
	}
	if filter.Kind != "" {
		fmt.Printf("  Kind:      %s\n", filter.Kind)
	}
	if filter.Namespace != "" {
		fmt.Printf("  Namespace: %s\n", filter.Namespace)
	}
	if filter.Name != "" {
		fmt.Printf("  Name:      %s\n", filter.Name)
	}
}

func printDebugFinalResourceStates(states map[string]*storage.ResourceLastState) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Resource\tType\tTimestamp\tSize")

	keys := make([]string, 0, len(states))
	for k := range states {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		state := states[key]
		// Convert string EventType to models.EventType
		var eventType models.EventType
		switch state.EventType {
		case "CREATE":
			eventType = models.EventTypeCreate
		case "UPDATE":
			eventType = models.EventTypeUpdate
		case "DELETE":
			eventType = models.EventTypeDelete
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			key,
			debugEventTypeString(eventType),
			time.Unix(0, state.Timestamp).Format("15:04:05"),
			formatDebugBytes(uint64(len(state.ResourceData))))
	}
	w.Flush()
}

func getDebugUniqueResources(events []*models.Event) map[string]*models.Event {
	unique := make(map[string]*models.Event)
	for _, event := range events {
		key := debugResourceKey(&event.Resource)
		if existing, ok := unique[key]; !ok || event.Timestamp > existing.Timestamp {
			unique[key] = event
		}
	}
	return unique
}

func debugResourceKey(res *models.ResourceMetadata) string {
	if res.Namespace != "" {
		return fmt.Sprintf("%s/%s/%s/%s/%s", res.Group, res.Version, res.Kind, res.Namespace, res.Name)
	}
	return fmt.Sprintf("%s/%s/%s/%s", res.Group, res.Version, res.Kind, res.Name)
}

func debugEventTypeString(t models.EventType) string {
	switch t {
	case models.EventTypeCreate:
		return "CREATE"
	case models.EventTypeUpdate:
		return "UPDATE"
	case models.EventTypeDelete:
		return "DELETE"
	default:
		return "UNKNOWN"
	}
}

func formatDebugBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGT"[exp])
}
