package embeddedstore

import "github.com/moolen/spectre/internal/models"

// recordPrefilter applies the group, version, kind and namespace parts of QueryFilters to
// raw record headers, so scans skip non-matching records before decoding them. It never
// rejects a record that filters.Matches would accept.
type recordPrefilter struct {
	group      string
	version    string
	kinds      map[string]struct{}
	namespaces map[string]struct{}
}

func newRecordPrefilter(filters models.QueryFilters) *recordPrefilter {
	prefilter := &recordPrefilter{group: filters.Group, version: filters.Version}
	if kinds := filters.GetKinds(); len(kinds) > 0 {
		prefilter.kinds = make(map[string]struct{}, len(kinds))
		for _, kind := range kinds {
			prefilter.kinds[kind] = struct{}{}
		}
	}
	if namespaces := filters.GetNamespaces(); len(namespaces) > 0 {
		prefilter.namespaces = make(map[string]struct{}, len(namespaces))
		for _, namespace := range namespaces {
			prefilter.namespaces[namespace] = struct{}{}
		}
	}
	if prefilter.group == "" && prefilter.version == "" && prefilter.kinds == nil && prefilter.namespaces == nil {
		return nil
	}
	return prefilter
}

func (f *recordPrefilter) matches(key *eventRecordKey) bool {
	if f == nil {
		return true
	}
	if f.group != "" && string(key.group) != f.group {
		return false
	}
	if f.version != "" && string(key.version) != f.version {
		return false
	}
	if f.kinds != nil {
		if _, ok := f.kinds[string(key.kind)]; !ok {
			return false
		}
	}
	if f.namespaces != nil {
		if _, ok := f.namespaces[string(key.namespace)]; !ok {
			return false
		}
	}
	return true
}
