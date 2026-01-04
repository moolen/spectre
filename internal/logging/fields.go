package logging

// cloneFields creates a copy of the source fields map.
// Returns a new map with all key-value pairs from src.
// Returns an empty map if src is nil or empty.
// This helper eliminates duplicate field copying logic.
func cloneFields(src map[string]interface{}) map[string]interface{} {
	if len(src) == 0 {
		return make(map[string]interface{})
	}
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
