package embedded

import (
	"encoding/json"
	"sort"
	"time"

	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/models"
)

type resourceKey struct {
	namespace string
	kind      string
	name      string
}

type resourceVersion struct {
	timestamp   int64
	eventType   models.EventType
	identity    graph.ResourceIdentity
	data        []byte
	object      map[string]any
	changeEvent analysisstore.ChangeEventInfo
}

type resourceRecord struct {
	uid      string
	versions []resourceVersion
}

func (r *resourceRecord) latestVersion() *resourceVersion {
	if len(r.versions) == 0 {
		return nil
	}
	return &r.versions[len(r.versions)-1]
}

func (r *resourceRecord) versionAt(timestamp int64) *resourceVersion {
	if len(r.versions) == 0 {
		return nil
	}
	idx := sort.Search(len(r.versions), func(i int) bool {
		return r.versions[i].timestamp > timestamp
	})
	if idx == 0 {
		return nil
	}
	return &r.versions[idx-1]
}

func (r *resourceRecord) activeVersionAt(timestamp int64) *resourceVersion {
	version := r.versionAt(timestamp)
	if version == nil || version.eventType == models.EventTypeDelete {
		return nil
	}
	return version
}

func (r *resourceRecord) visibleVersionWithinWindow(failureTimestampNs, windowStartNs int64) *resourceVersion {
	version := r.versionAt(failureTimestampNs)
	if version == nil {
		return nil
	}
	if version.eventType != models.EventTypeDelete {
		return version
	}
	if version.timestamp >= windowStartNs {
		return version
	}
	return nil
}

type snapshot struct {
	resourcesByUID         map[string]*resourceRecord
	resourcesByKey         map[resourceKey][]*resourceRecord
	k8sEventsByInvolvedUID map[string][]analysisstore.K8sEventInfo
}

func (s *snapshot) resolveByName(namespace, kind, name string, failureTimestampNs, windowStartNs int64) *resourceVersion {
	records := s.resourcesByKey[resourceKey{namespace: namespace, kind: kind, name: name}]
	var deletedCandidate *resourceVersion
	for _, record := range records {
		if active := record.activeVersionAt(failureTimestampNs); active != nil {
			return active
		}
		if windowStartNs == 0 {
			continue
		}
		if deleted := record.visibleVersionWithinWindow(failureTimestampNs, windowStartNs); deleted != nil && deleted.eventType == models.EventTypeDelete {
			if deletedCandidate == nil || deleted.timestamp > deletedCandidate.timestamp {
				deletedCandidate = deleted
			}
		}
	}
	return deletedCandidate
}

func (s *snapshot) activeResourcesInNamespace(namespace string, timestampNs int64) []*resourceVersion {
	var result []*resourceVersion
	for _, record := range s.resourcesByUID {
		version := record.activeVersionAt(timestampNs)
		if version == nil || version.identity.Kind == "Event" || version.identity.Namespace != namespace {
			continue
		}
		result = append(result, version)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].identity.Kind != result[j].identity.Kind {
			return result[i].identity.Kind < result[j].identity.Kind
		}
		if result[i].identity.Name != result[j].identity.Name {
			return result[i].identity.Name < result[j].identity.Name
		}
		return result[i].identity.UID < result[j].identity.UID
	})
	return result
}

func copyIdentity(version *resourceVersion) graph.ResourceIdentity {
	identity := version.identity
	identity.Labels = copyStringMap(identity.Labels)
	return identity
}

func copyStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}

func parseObject(data []byte) map[string]any {
	if len(data) == 0 {
		return nil
	}
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		return nil
	}
	return object
}

func timeFromNs(value int64) time.Time {
	return time.Unix(0, value)
}
