package embeddedstore

import (
	"time"

	"github.com/moolen/spectre/internal/models"
)

// k8sEventHashHorizon bounds how long the payload hash of a core/v1 Event is
// remembered for duplicate detection. Kubernetes keeps Events for one hour by
// default, so a re-list can only repeat Events about that old.
const k8sEventHashHorizon = 2 * time.Hour

type k8sEventPayloadHash struct {
	hash      uint64
	timestamp int64
}

// dropUnchangedEvents removes Create and Update events whose payload is
// byte-identical to the latest state already recorded for the same object.
//
// The watcher re-lists a resource type whenever its watch stream has to be
// re-established and reports every listed object again. On a large cluster that
// repeats every unchanged object several times an hour; keeping those copies
// would multiply the retained history of long retention windows without adding
// information. A real modification always changes metadata.resourceVersion, so
// identical bytes can only be a repeat.
//
// The input slice is returned unchanged when nothing is dropped.
func (p *Projection) dropUnchangedEvents(events []models.Event) ([]models.Event, int) {
	if p == nil || len(events) == 0 {
		return events, 0
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	// Earlier events of the same batch are not applied yet; they supersede the
	// projection's latest state for their object.
	var batchLatest map[string]batchObjectState
	if len(events) > 1 {
		batchLatest = make(map[string]batchObjectState, len(events))
	}

	var kept []models.Event
	dropped := 0
	for i := range events {
		event := &events[i]
		hash := payloadHash(event.Data)
		unchanged := false
		if state, ok := batchLatest[event.Resource.UID]; ok {
			unchanged = isRepeatOf(event, hash, state.hash, state.deleted)
		} else {
			unchanged = p.isUnchangedLocked(event, hash)
		}
		if batchLatest != nil && event.Resource.UID != "" {
			batchLatest[event.Resource.UID] = batchObjectState{hash: hash, deleted: event.Type == models.EventTypeDelete}
		}
		if unchanged {
			if kept == nil {
				kept = make([]models.Event, i, len(events)-1)
				copy(kept, events[:i])
			}
			dropped++
			continue
		}
		if kept != nil {
			kept = append(kept, *event)
		}
	}
	if kept == nil {
		return events, 0
	}
	return kept, dropped
}

type batchObjectState struct {
	hash    uint64
	deleted bool
}

// isRepeatOf reports whether event is a Create/Update carrying exactly the payload
// of a stored, non-deleted state with hash latestHash.
func isRepeatOf(event *models.Event, hash, latestHash uint64, latestDeleted bool) bool {
	if event.Type == models.EventTypeDelete || hash == 0 || event.Resource.UID == "" {
		return false
	}
	return !latestDeleted && latestHash != 0 && latestHash == hash
}

func (p *Projection) isUnchangedLocked(event *models.Event, hash uint64) bool {
	if event.Resource.Kind == "Event" {
		seen, ok := p.k8sEventHashes[event.Resource.UID]
		return ok && isRepeatOf(event, hash, seen.hash, false)
	}
	record := p.resourcesByUID[event.Resource.UID]
	if record == nil {
		return false
	}
	latest := record.latestVersion()
	if latest == nil {
		return false
	}
	return isRepeatOf(event, hash, latest.dataHash, latest.isDelete())
}

// rememberK8sEventHashLocked records the payload hash of a core/v1 Event, which has
// no resource record to compare against, and forgets hashes past the horizon.
func (p *Projection) rememberK8sEventHashLocked(event models.Event) {
	if event.Resource.UID == "" || len(event.Data) == 0 {
		return
	}
	if p.k8sEventHashes == nil {
		p.k8sEventHashes = make(map[string]k8sEventPayloadHash)
	}
	p.k8sEventHashes[event.Resource.UID] = k8sEventPayloadHash{
		hash:      payloadHash(event.Data),
		timestamp: event.Timestamp,
	}
	if len(p.k8sEventHashes) < p.k8sEventHashPruneAt {
		return
	}
	cutoff := event.Timestamp - int64(k8sEventHashHorizon)
	for uid, seen := range p.k8sEventHashes {
		if seen.timestamp < cutoff {
			delete(p.k8sEventHashes, uid)
		}
	}
	p.k8sEventHashPruneAt = max(1024, 2*len(p.k8sEventHashes))
}
