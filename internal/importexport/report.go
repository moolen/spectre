package importexport

import (
	"fmt"
	"strings"
)

// FormatImportReport formats an ImportReport for terminal display
func FormatImportReport(report *ImportReport) string {
	var sb strings.Builder

	sb.WriteString("Import Summary:\n")
	sb.WriteString(fmt.Sprintf("  Total Events:   %d\n", report.TotalEvents))
	sb.WriteString(fmt.Sprintf("  Merged Hours:   %d\n", report.MergedHours))
	sb.WriteString(fmt.Sprintf("  Imported Files: %d\n", report.ImportedFiles))
	sb.WriteString(fmt.Sprintf("  Duration:       %s\n", report.Duration))

	if report.TotalFiles > 0 {
		sb.WriteString(fmt.Sprintf("  Total Files:    %d\n", report.TotalFiles))
	}
	if report.SkippedFiles > 0 {
		sb.WriteString(fmt.Sprintf("  Skipped Files:  %d\n", report.SkippedFiles))
	}
	if report.FailedFiles > 0 {
		sb.WriteString(fmt.Sprintf("  Failed Files:   %d\n", report.FailedFiles))
	}

	if len(report.Errors) > 0 {
		sb.WriteString("\nErrors:\n")
		for _, err := range report.Errors {
			sb.WriteString(fmt.Sprintf("  - %s\n", err))
		}
	}

	return sb.String()
}
