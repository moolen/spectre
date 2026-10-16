package embeddedstore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/moolen/spectre/internal/models"
)

// maxPinnedSegments bounds how many pinned segments accumulate before they are merged
// into one containing only the versions still referenced.
const maxPinnedSegments = 4

const pinnedSegmentPrefix = "pin-"

// applySegmentRetentionLocked drops segment records older than cutoff while keeping the
// payloads the projection still needs: for every resource, the version it was in when
// the retained window starts. Those are copied into a pinned segment first, then the
// expired active segments are rewritten or retired, and finally the projection is
// pruned. Retired segments are deleted by the next checkpoint.
func (e *Engine) applySegmentRetentionLocked(ctx context.Context, cutoffTimestamp int64) error {
	if cutoffTimestamp <= 0 {
		return nil
	}

	expired := make(map[string]bool)
	for _, segment := range e.manifest.ActiveSegments {
		if segment.EventCount == 0 || segment.MinTimestamp < cutoffTimestamp {
			expired[segment.ID] = true
		}
	}

	updatedManifest := e.manifest
	retired := make([]RetiredSegment, 0)
	var pinnedID string
	var pinnedLocations []segmentRecordLocation

	if len(expired) > 0 {
		events := e.projection.pinnedVersionEvents(cutoffTimestamp, func(handle *segmentDataHandle) bool {
			return expired[handle.id]
		})
		if len(events) > 0 {
			pinnedID = pinnedSegmentPrefix + newSegmentID(e.nextHighWaterMark)
			meta, locations, err := writeSegmentLocated(e.rootDir, pinnedID, events)
			if err != nil {
				return fmt.Errorf("write pinned segment: %w", err)
			}
			pinnedLocations = locations
			updatedManifest.PinnedSegments = append(append([]SegmentMeta(nil), updatedManifest.PinnedSegments...), segmentMetaFromBundle(meta, e.nextHighWaterMark))
		}
	}

	rewrittenLocations := make(map[string][]segmentRecordLocation)
	activeSegments := make([]SegmentMeta, 0, len(updatedManifest.ActiveSegments))
	for _, segment := range updatedManifest.ActiveSegments {
		if !expired[segment.ID] {
			activeSegments = append(activeSegments, segment)
			continue
		}
		replacedBy := []string{}
		if pinnedID != "" {
			replacedBy = append(replacedBy, pinnedID)
		}
		if segment.MaxTimestamp >= cutoffTimestamp {
			replacementID := newSegmentID(segment.HighWaterMark)
			meta, locations, err := rewriteSegmentFrom(ctx, e.rootDir, segment.ID, replacementID, func(record *rawSegmentRecord) bool {
				return record.event.Timestamp >= cutoffTimestamp
			})
			if err != nil {
				return fmt.Errorf("rewrite segment %q for retention: %w", segment.ID, err)
			}
			if meta.EventCount > 0 {
				activeSegments = append(activeSegments, segmentMetaFromBundle(meta, segment.HighWaterMark))
				rewrittenLocations[replacementID] = locations
				replacedBy = append(replacedBy, replacementID)
			} else {
				_ = os.RemoveAll(filepath.Join(e.rootDir, segmentsDirName, replacementID))
			}
		}
		retired = append(retired, RetiredSegment{ID: segment.ID, ReplacedBy: replacedBy})
	}
	updatedManifest.ActiveSegments = activeSegments
	updatedManifest.RetiredSegments = append(append([]RetiredSegment(nil), updatedManifest.RetiredSegments...), retired...)

	if err := storeManifest(e.rootDir, updatedManifest); err != nil {
		return fmt.Errorf("store manifest: %w", err)
	}
	readers, err := openActiveSegmentReaders(e.rootDir, updatedManifest.ActiveSegments)
	if err != nil {
		return fmt.Errorf("reopen retained segments: %w", err)
	}
	e.manifest = updatedManifest
	e.segmentReaders = readers

	if pinnedID != "" {
		e.projection.AttachSegmentLocations(pinnedID, pinnedLocations)
	}
	for id, locations := range rewrittenLocations {
		e.projection.AttachSegmentLocations(id, locations)
	}
	retiredIDs := make([]string, 0, len(retired))
	for i := range retired {
		retiredIDs = append(retiredIDs, retired[i].ID)
	}
	e.projection.ReleaseSegmentHandles(retiredIDs)
	e.projection.ApplyRetentionCutoff(cutoffTimestamp)

	return e.compactPinnedSegmentsLocked()
}

// compactPinnedSegmentsLocked rewrites the pinned segments into one that only holds
// versions the projection still references, once enough of them have accumulated.
func (e *Engine) compactPinnedSegmentsLocked() error {
	if len(e.manifest.PinnedSegments) <= maxPinnedSegments {
		return nil
	}
	pinned := make(map[string]bool, len(e.manifest.PinnedSegments))
	for _, segment := range e.manifest.PinnedSegments {
		pinned[segment.ID] = true
	}
	events := e.projection.referencedVersionEvents(func(handle *segmentDataHandle) bool {
		return pinned[handle.id]
	})

	updatedManifest := e.manifest
	updatedManifest.PinnedSegments = nil
	var newID string
	var locations []segmentRecordLocation
	if len(events) > 0 {
		newID = pinnedSegmentPrefix + newSegmentID(e.nextHighWaterMark)
		meta, located, err := writeSegmentLocated(e.rootDir, newID, events)
		if err != nil {
			return fmt.Errorf("compact pinned segments: %w", err)
		}
		locations = located
		updatedManifest.PinnedSegments = []SegmentMeta{segmentMetaFromBundle(meta, e.nextHighWaterMark)}
	}
	retiredIDs := make([]string, 0, len(e.manifest.PinnedSegments))
	for _, segment := range e.manifest.PinnedSegments {
		replacedBy := []string{}
		if newID != "" {
			replacedBy = append(replacedBy, newID)
		}
		updatedManifest.RetiredSegments = append(updatedManifest.RetiredSegments, RetiredSegment{ID: segment.ID, ReplacedBy: replacedBy})
		retiredIDs = append(retiredIDs, segment.ID)
	}
	if err := storeManifest(e.rootDir, updatedManifest); err != nil {
		return fmt.Errorf("compact pinned segments: store manifest: %w", err)
	}
	e.manifest = updatedManifest
	if newID != "" {
		e.projection.AttachSegmentLocations(newID, locations)
	}
	e.projection.ReleaseSegmentHandles(retiredIDs)
	return nil
}

// rewriteSegmentFrom copies the records of one segment accepted by keep into a new
// segment, copying binary frames verbatim.
func rewriteSegmentFrom(ctx context.Context, rootDir, sourceID, targetID string, keep func(*rawSegmentRecord) bool) (segmentBundleMeta, []segmentRecordLocation, error) {
	builder, err := newSegmentBuilder(rootDir, targetID)
	if err != nil {
		return segmentBundleMeta{}, nil, err
	}
	if err := mergeSegmentRecords([]string{segmentEventsPath(rootDir, sourceID)}, func(record *rawSegmentRecord) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !keep(record) {
			return nil
		}
		return builder.AppendRecord(record)
	}); err != nil {
		builder.Abort()
		return segmentBundleMeta{}, nil, err
	}
	meta, err := builder.Finish()
	if err != nil {
		return segmentBundleMeta{}, nil, err
	}
	return meta, builder.Locations(), nil
}

// pinnedVersionEvents materialises the versions older than cutoff that retention keeps
// and whose payload lives in a segment selected by inSegment.
func (p *Projection) pinnedVersionEvents(cutoffTimestamp int64, inSegment func(*segmentDataHandle) bool) []models.Event {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var events []models.Event
	for _, record := range p.resourcesByUID {
		for _, version := range retainedVersionsBefore(record, cutoffTimestamp) {
			if !version.ref.valid() || !inSegment(version.ref.segment) {
				continue
			}
			events = append(events, versionAsEvent(version))
		}
	}
	return events
}

// referencedVersionEvents materialises every version whose payload lives in a segment
// selected by inSegment.
func (p *Projection) referencedVersionEvents(inSegment func(*segmentDataHandle) bool) []models.Event {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var events []models.Event
	for _, record := range p.resourcesByUID {
		for i := range record.versions {
			version := &record.versions[i]
			if !version.ref.valid() || !inSegment(version.ref.segment) {
				continue
			}
			events = append(events, versionAsEvent(version))
		}
	}
	return events
}

func versionAsEvent(version *resourceVersion) models.Event {
	return models.Event{
		ID:        version.eventID(),
		Timestamp: version.timestamp,
		Type:      version.eventType(),
		Resource:  resourceMetadataFromIdentity(version.identity()),
		Data:      versionPayload(version),
	}
}
