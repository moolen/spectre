package sync

import "context"

// BatchProcessingOptions applies per-call overrides to ProcessBatch behavior.
// This is used for explicit startup-import tuning without changing the default pipeline config.
type BatchProcessingOptions struct {
	DisableCausality bool
	TimelineOnly     bool
}

type batchProcessingOptionsKey struct{}

// WithBatchProcessingOptions attaches batch processing overrides to the context.
func WithBatchProcessingOptions(ctx context.Context, opts BatchProcessingOptions) context.Context {
	return context.WithValue(ctx, batchProcessingOptionsKey{}, opts)
}

func batchProcessingOptionsFromContext(ctx context.Context) BatchProcessingOptions {
	if ctx == nil {
		return BatchProcessingOptions{}
	}

	if opts, ok := ctx.Value(batchProcessingOptionsKey{}).(BatchProcessingOptions); ok {
		return opts
	}

	return BatchProcessingOptions{}
}
