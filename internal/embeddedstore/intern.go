package embeddedstore

import "unique"

// internString returns the canonical copy of s. Kinds, API versions, namespaces, label
// keys and values and Event reasons repeat across hundreds of thousands of retained
// records; decoding gives each record its own copy unless they are interned.
func internString(s string) string {
	if s == "" {
		return ""
	}
	return unique.Make(s).Value()
}

// internLabels returns a copy of labels with interned keys and values.
func internLabels(labels map[string]string) map[string]string {
	if len(labels) == 0 {
		return labels
	}
	interned := make(map[string]string, len(labels))
	for key, value := range labels {
		interned[internString(key)] = internString(value)
	}
	return interned
}
