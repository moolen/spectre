package logprocessing

import (
	"regexp"
	"strings"
)

// Regex patterns compiled once at package initialization
var (
	// IP addresses
	ipv4Pattern = regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`)
	ipv6Pattern = regexp.MustCompile(`\b[0-9a-fA-F:]+:[0-9a-fA-F:]+\b`)

	// UUIDs (standard format)
	uuidPattern = regexp.MustCompile(`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`)

	// Timestamps (ISO8601, RFC3339, Unix timestamps)
	timestampPattern     = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})?\b`)
	unixTimestampPattern = regexp.MustCompile(`\b\d{10,13}\b`)

	// Hex strings (0x prefix or long hex sequences)
	hexPattern     = regexp.MustCompile(`\b0x[0-9a-fA-F]+\b`)
	longHexPattern = regexp.MustCompile(`\b[0-9a-fA-F]{16,}\b`)

	// File paths (Unix and Windows)
	filePathPattern    = regexp.MustCompile(`(/[a-zA-Z0-9_.-]+)+`)
	windowsPathPattern = regexp.MustCompile(`[A-Z]:\\[a-zA-Z0-9_.\-\\]+`)

	// URLs
	urlPattern = regexp.MustCompile(`\bhttps?://[a-zA-Z0-9.-]+[a-zA-Z0-9/._?=&-]*\b`)

	// Email addresses
	emailPattern = regexp.MustCompile(`\b[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}\b`)
)

// AggressiveMask applies all masking patterns to a template.
// Applies patterns in specific order (specific before generic).
// Preserves HTTP status codes per user decision from CONTEXT.md:
// "returned 404 vs returned 500 stay distinct"
func AggressiveMask(template string) string {
	// Apply patterns in specific order (specific before generic)
	template = ipv6Pattern.ReplaceAllString(template, "<IP>")
	template = ipv4Pattern.ReplaceAllString(template, "<IP>")
	template = uuidPattern.ReplaceAllString(template, "<UUID>")
	template = timestampPattern.ReplaceAllString(template, "<TIMESTAMP>")
	template = unixTimestampPattern.ReplaceAllString(template, "<TIMESTAMP>")
	template = hexPattern.ReplaceAllString(template, "<HEX>")
	template = longHexPattern.ReplaceAllString(template, "<HEX>")
	template = urlPattern.ReplaceAllString(template, "<URL>")
	template = emailPattern.ReplaceAllString(template, "<EMAIL>")
	template = filePathPattern.ReplaceAllString(template, "<PATH>")
	template = windowsPathPattern.ReplaceAllString(template, "<PATH>")

	// Apply Kubernetes-specific masking
	template = MaskKubernetesNames(template)

	// Mask generic numbers but preserve HTTP status codes
	template = maskNumbersExceptStatusCodes(template)

	return template
}

// maskNumbersExceptStatusCodes masks numbers but preserves HTTP status codes.
// User decision from CONTEXT.md: "HTTP status codes preserved as literals"
func maskNumbersExceptStatusCodes(template string) string {
	// Status code context keywords
	preserveContexts := []string{
		"status", "code", "http", "returned", "response",
	}

	// Split into tokens for context-aware masking
	tokens := strings.Fields(template)

	for i, token := range tokens {
		// Check if token is a number
		if isNumber(token) {
			shouldMask := true

			// Check surrounding 3 tokens for status code context
			windowStart := max(0, i-3)
			windowEnd := min(len(tokens), i+4)

			for j := windowStart; j < windowEnd; j++ {
				if j == i {
					continue // Skip the token itself
				}
				lower := strings.ToLower(tokens[j])
				for _, ctx := range preserveContexts {
					if strings.Contains(lower, ctx) {
						shouldMask = false
						break
					}
				}
				if !shouldMask {
					break
				}
			}

			if shouldMask {
				tokens[i] = "<NUM>"
			}
		}
	}

	return strings.Join(tokens, " ")
}

// isNumber checks if a string represents a number
func isNumber(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
