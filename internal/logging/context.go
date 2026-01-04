package logging

import "context"

// Context keys for trace and span IDs
type contextKey string

const (
	traceIDKey contextKey = "trace_id"
	spanIDKey  contextKey = "span_id"
)

// TraceIDKey returns the context key for trace ID.
// Use this to add a trace ID to a context:
//
//	ctx := context.WithValue(ctx, logging.TraceIDKey(), "trace-123")
func TraceIDKey() interface{} {
	return traceIDKey
}

// SpanIDKey returns the context key for span ID.
// Use this to add a span ID to a context:
//
//	ctx := context.WithValue(ctx, logging.SpanIDKey(), "span-456")
func SpanIDKey() interface{} {
	return spanIDKey
}

// extractContextFields extracts trace_id and span_id from context if available.
// Returns nil if context is nil or if no trace/span IDs are found.
func extractContextFields(ctx context.Context) map[string]interface{} {
	if ctx == nil {
		return nil
	}

	fields := make(map[string]interface{})

	if traceID := ctx.Value(traceIDKey); traceID != nil {
		fields["trace_id"] = traceID
	}

	if spanID := ctx.Value(spanIDKey); spanID != nil {
		fields["span_id"] = spanID
	}

	if len(fields) == 0 {
		return nil
	}

	return fields
}
