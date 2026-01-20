package namespacegraph

import "time"

const (
	// DefaultLimit is the default number of resources per page
	// Set to 50 for faster initial loads with lazy loading
	DefaultLimit = 50

	// MaxLimit is the maximum number of resources per page
	MaxLimit = 500

	// DefaultMaxDepth is the default maximum relationship traversal depth
	// Set to 1 for performance - deeper traversals are expensive
	DefaultMaxDepth = 1

	// MaxMaxDepth is the maximum allowed traversal depth
	MaxMaxDepth = 10

	// MinMaxDepth is the minimum allowed traversal depth
	MinMaxDepth = 1

	// CausalPathMaxDepth is the depth used for causal path discovery
	// This needs to be higher than DefaultMaxDepth to traverse full chains like:
	// HelmRelease -> Deployment -> ReplicaSet -> Pod -> Node (5 hops)
	// Plus any referenced ConfigMaps/Secrets adds more hops
	CausalPathMaxDepth = 7

	// DefaultLookback is the default lookback window for anomaly/causal analysis
	DefaultLookback = 30 * time.Minute

	// MaxLookback is the maximum allowed lookback window
	MaxLookback = 24 * time.Hour

	// QueryTimeoutMs is the timeout for graph queries in milliseconds
	// Set to 120 seconds to accommodate resource-constrained environments
	QueryTimeoutMs = 120000

	// Status constants - matching internal/analyzer/status.go values
	StatusReady       = "Ready"
	StatusWarning     = "Warning"
	StatusError       = "Error"
	StatusTerminating = "Terminating"
	StatusUnknown     = "Unknown"
)
