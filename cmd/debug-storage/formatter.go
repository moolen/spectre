package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/moolen/spectre/internal/models"
)

// formatSummary outputs a summary view of the storage file
func formatSummary(fileData *FileData, filteredEvents []*models.Event, filter *EventFilter) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	// File Header Info
	fmt.Fprintf(w, "\n%s\n", strings.Repeat("=", 80))
	fmt.Fprintf(w, "FILE HEADER\n")
	fmt.Fprintf(w, "%s\n", strings.Repeat("=", 80))
	fmt.Fprintf(w, "Format Version:\t%s\n", fileData.Header.FormatVersion)
	fmt.Fprintf(w, "Compression:\t%s\n", fileData.Header.CompressionAlgorithm)
	fmt.Fprintf(w, "Encoding:\t%s\n", fileData.Header.EncodingFormat)
	fmt.Fprintf(w, "Block Size:\t%s\n", formatBytes(int64(fileData.Header.BlockSize)))
	fmt.Fprintf(w, "Checksum Enabled:\t%v\n", fileData.Header.ChecksumEnabled)
	fmt.Fprintf(w, "Created:\t%s\n", time.Unix(0, fileData.Header.CreatedAt).Format(time.RFC3339))

	// File Statistics
	fmt.Fprintf(w, "\n%s\n", strings.Repeat("=", 80))
	fmt.Fprintf(w, "FILE STATISTICS\n")
	fmt.Fprintf(w, "%s\n", strings.Repeat("=", 80))
	fmt.Fprintf(w, "Total Blocks:\t%d\n", fileData.Statistics["TotalBlocks"])
	fmt.Fprintf(w, "Total Events:\t%d\n", len(fileData.Events))
	fmt.Fprintf(w, "Compressed Size:\t%s\n", formatBytes(fileData.Statistics["CompressedSize"].(int64)))
	fmt.Fprintf(w, "Uncompressed Size:\t%s\n", formatBytes(fileData.Statistics["UncompressedSize"].(int64)))
	ratio := fileData.Statistics["CompressionRatio"].(float64)
	fmt.Fprintf(w, "Compression Ratio:\t%.1f%%\n", ratio*100)

	// Time range
	minTime := time.Unix(0, fileData.Statistics["TimestampMin"].(int64)).Format(time.RFC3339)
	maxTime := time.Unix(0, fileData.Statistics["TimestampMax"].(int64)).Format(time.RFC3339)
	fmt.Fprintf(w, "Time Range:\t%s to %s\n", minTime, maxTime)

	// Index Statistics
	fmt.Fprintf(w, "\n%s\n", strings.Repeat("=", 80))
	fmt.Fprintf(w, "INDEX STATISTICS\n")
	fmt.Fprintf(w, "%s\n", strings.Repeat("=", 80))
	fmt.Fprintf(w, "Unique Kinds:\t%d\n", fileData.Statistics["UniqueKinds"])
	fmt.Fprintf(w, "Unique Namespaces:\t%d\n", fileData.Statistics["UniqueNamespaces"])
	fmt.Fprintf(w, "Unique Groups:\t%d\n", fileData.Statistics["UniqueGroups"])
	fmt.Fprintf(w, "Final Resource States:\t%d\n", len(fileData.FinalResourceStates))

	// Block Summary
	fmt.Fprintf(w, "\n%s\n", strings.Repeat("=", 80))
	fmt.Fprintf(w, "BLOCKS SUMMARY\n")
	fmt.Fprintf(w, "%s\n", strings.Repeat("=", 80))
	fmt.Fprintf(w, "ID\tOffset\tComp Size\tUncomp Size\tEvents\tKinds\tNamespaces\tGroups\n")

	for _, block := range fileData.Blocks {
		meta := block.Metadata
		compSize := formatBytes(int64(meta.CompressedLength))
		uncompSize := formatBytes(int64(meta.UncompressedLength))
		kindCount := len(meta.KindSet)
		nsCount := len(meta.NamespaceSet)
		groupCount := len(meta.GroupSet)
		fmt.Fprintf(w, "%d\t%d\t%v\t%v\t%d\t%d\t%d\t%d\n",
			meta.ID, meta.Offset, compSize, uncompSize, meta.EventCount, kindCount, nsCount, groupCount)
	}

	w.Flush()

	// Filtered Results
	if !isEmptyFilter(filter) {
		fmt.Printf("\n%s\n", strings.Repeat("=", 80))
		fmt.Printf("FILTER RESULTS\n")
		fmt.Printf("%s\n", strings.Repeat("=", 80))
		printFilterInfo(filter)
		fmt.Printf("\nMatching events: %d\n", len(filteredEvents))

		if len(filteredEvents) > 0 {
			uniqueResources := getUniqueResources(filteredEvents)
			fmt.Printf("Unique resources: %d\n\n", len(uniqueResources))

			// Print resource summary table
			w = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "Group\tVersion\tKind\tNamespace\tName\tEvent Type\tTimestamp\n")
			fmt.Fprintf(w, "%s\n", strings.Repeat("-", 100))

			// Sort resources by key for consistent output
			keys := make([]string, 0, len(uniqueResources))
			for k := range uniqueResources {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			for _, key := range keys {
				event := uniqueResources[key]
				res := event.Resource
				ts := time.Unix(0, event.Timestamp).Format(time.RFC3339)
				eventType := eventTypeString(event.Type)
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					res.Group, res.Version, res.Kind, res.Namespace, res.Name, eventType, ts)
			}
			w.Flush()
		}
	}

	fmt.Printf("\n")
	return nil
}

// formatExtended outputs detailed view of the storage file
func formatExtended(fileData *FileData, filteredEvents []*models.Event, filter *EventFilter) error {
	// First print summary
	if err := formatSummary(fileData, filteredEvents, filter); err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Detailed block information
	fmt.Printf("%s\n", strings.Repeat("=", 80))
	fmt.Printf("DETAILED BLOCK INFORMATION\n")
	fmt.Printf("%s\n", strings.Repeat("=", 80))
	defer w.Flush()

	for _, block := range fileData.Blocks {
		meta := block.Metadata
		fmt.Printf("\nBlock %d (Offset: %d)\n", meta.ID, meta.Offset)
		fmt.Printf("  Compressed Size: %s\n", formatBytes(int64(meta.CompressedLength)))
		fmt.Printf("  Uncompressed Size: %s\n", formatBytes(int64(meta.UncompressedLength)))
		fmt.Printf("  Events: %d\n", meta.EventCount)
		fmt.Printf("  Checksum: %s\n", meta.Checksum)
		fmt.Printf("  Time Range: %s to %s\n",
			time.Unix(0, meta.TimestampMin).Format(time.RFC3339),
			time.Unix(0, meta.TimestampMax).Format(time.RFC3339))

		// Print kinds, namespaces, groups
		if len(meta.KindSet) > 0 {
			kinds := make([]string, len(meta.KindSet))
			copy(kinds, meta.KindSet)
			sort.Strings(kinds)
			fmt.Printf("  Kinds: %s\n", strings.Join(kinds, ", "))
		}

		if len(meta.NamespaceSet) > 0 {
			namespaces := make([]string, len(meta.NamespaceSet))
			copy(namespaces, meta.NamespaceSet)
			sort.Strings(namespaces)
			fmt.Printf("  Namespaces: %s\n", strings.Join(namespaces, ", "))
		}

		if len(meta.GroupSet) > 0 {
			groups := make([]string, len(meta.GroupSet))
			copy(groups, meta.GroupSet)
			sort.Strings(groups)
			fmt.Printf("  Groups: %s\n", strings.Join(groups, ", "))
		}
	}

	// Detailed event information
	fmt.Printf("\n%s\n", strings.Repeat("=", 80))
	fmt.Printf("DETAILED EVENT INFORMATION\n")
	fmt.Printf("%s\n", strings.Repeat("=", 80))

	events := filteredEvents
	if isEmptyFilter(filter) {
		events = fileData.Events
	}

	if len(events) == 0 {
		fmt.Printf("No events to display\n")
	} else {
		// Print event table
		w = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "ID\tTimestamp\tEvent Type\tGroup\tVersion\tKind\tNamespace\tName\tData Size\n")
		fmt.Fprintf(w, "%s\n", strings.Repeat("-", 150))

		for _, event := range events {
			res := &event.Resource
			ts := time.Unix(0, event.Timestamp).Format(time.RFC3339)
			eventType := eventTypeString(event.Type)
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				event.ID, ts, eventType, res.Group, res.Version, res.Kind, res.Namespace, res.Name,
				formatBytes(int64(event.DataSize)))
		}
		w.Flush()
	}

	// Display final resource states (only in extended mode)
	if err := formatFinalResourceStates(fileData); err != nil {
		return err
	}

	return nil
}

// formatJSON outputs the file data as JSON
func formatJSON(fileData *FileData, filteredEvents []*models.Event) error {
	output := map[string]interface{}{
		"header":     fileData.Header,
		"statistics": fileData.Statistics,
		"blocks":     len(fileData.Blocks),
		"events":     filteredEvents,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// formatFinalResourceStates displays the final state of all resources at file close time
func formatFinalResourceStates(fileData *FileData) error {
	if len(fileData.FinalResourceStates) == 0 {
		fmt.Printf("\n(No final resource states recorded)\n")
		return nil
	}

	fmt.Printf("\n%s\n", strings.Repeat("=", 80))
	fmt.Printf("FINAL RESOURCE STATES\n")
	fmt.Printf("(Consistent view of resources at file close time)\n")
	fmt.Printf("%s\n", strings.Repeat("=", 80))

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Resource Key\tEvent Type\tTimestamp\tData Size\n")
	fmt.Fprintf(w, "%s\n", strings.Repeat("-", 120))

	// Sort by key for consistent output
	keys := make([]string, 0, len(fileData.FinalResourceStates))
	for k := range fileData.FinalResourceStates {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		state := fileData.FinalResourceStates[key]
		ts := time.Unix(0, state.Timestamp).Format(time.RFC3339)
		dataSize := len(state.ResourceData)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			key, state.EventType, ts, formatBytes(int64(dataSize)))
	}
	w.Flush()

	fmt.Printf("\nTotal resources with final state: %d\n", len(fileData.FinalResourceStates))

	return nil
}

// Helper functions

func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func eventTypeString(et models.EventType) string {
	switch et {
	case models.EventTypeCreate:
		return "CREATE"
	case models.EventTypeUpdate:
		return "UPDATE"
	case models.EventTypeDelete:
		return "DELETE"
	default:
		return fmt.Sprintf("UNKNOWN(%s)", et)
	}
}

func printFilterInfo(filter *EventFilter) {
	var filters []string
	if filter.Group != "" {
		filters = append(filters, fmt.Sprintf("group=%s", filter.Group))
	}
	if filter.Version != "" {
		filters = append(filters, fmt.Sprintf("version=%s", filter.Version))
	}
	if filter.Kind != "" {
		filters = append(filters, fmt.Sprintf("kind=%s", filter.Kind))
	}
	if filter.Namespace != "" {
		filters = append(filters, fmt.Sprintf("namespace=%s", filter.Namespace))
	}
	if filter.Name != "" {
		filters = append(filters, fmt.Sprintf("name=%s", filter.Name))
	}
	if len(filters) > 0 {
		fmt.Printf("Active filters: %s\n", strings.Join(filters, ", "))
	}
}
