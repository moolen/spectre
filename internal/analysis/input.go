package analysis

// PrepareAnalyzeInput applies package-level defaults before analysis executes.
func PrepareAnalyzeInput(input AnalyzeInput) AnalyzeInput {
	if input.LookbackNs == 0 {
		input.LookbackNs = DefaultLookbackNs
	}
	if input.Format == "" {
		input.Format = FormatDiff
	}

	return input
}
