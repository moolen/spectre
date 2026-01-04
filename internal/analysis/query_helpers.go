package analysis

// extractUIDs extracts UIDs from the ownership chain for batch querying.
// This is a helper function to convert []ResourceWithDistance to []string for use
// in query parameters.
func extractUIDs(chain []ResourceWithDistance) []string {
	uids := make([]string, len(chain))
	for i, rwd := range chain {
		uids[i] = rwd.Resource.UID
	}
	return uids
}
