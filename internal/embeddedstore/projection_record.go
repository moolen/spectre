package embeddedstore

import (
	"sort"

	"github.com/moolen/spectre/internal/models"
)

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

// latestChangeState returns the change state of the newest version, deriving it from the
// stored payload when the record was restored without one (older checkpoints).
func (r *resourceRecord) latestChangeState() recordChangeState {
	if r == nil || len(r.versions) == 0 {
		return recordChangeState{known: true}
	}
	if r.latest.known {
		return r.latest
	}
	latest := &r.versions[len(r.versions)-1]
	data := versionPayload(latest)
	r.latest = newRecordChangeState(data, parseObject(data))
	return r.latest
}
