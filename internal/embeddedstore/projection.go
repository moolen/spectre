package embeddedstore

import (
	"sync"

	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/models"
)

type resourceKey struct {
	namespace string
	kind      string
	name      string
}

type orderedResourceKey struct {
	kind      string
	namespace string
	name      string
	uid       string
}

type recentResourceChange struct {
	timestamp int64
	uid       string
}

// resourceVersion is one observed state of a resource. It is kept compact because the
// projection holds one per event for the whole retention window: identity strings live
// in a versionMeta shared by consecutive versions, and identity/change info structs are
// derived on demand (see version_compact.go).
type resourceVersion struct {
	eventID   string
	timestamp int64
	meta      *versionMeta
	labels    map[string]string
	// data holds the payload until the event is flushed; afterwards ref locates it.
	// Read payloads through versionPayload rather than these fields.
	data     []byte
	ref      versionDataRef
	dataHash uint64
	typeCode uint8
	// statusCode indexes versionStatuses; flags holds versionFlag* bits.
	statusCode uint8
	flags      uint8
}

type resourceRecord struct {
	uid      string
	versions []resourceVersion
	// latest summarises versions[len-1] for classifying the next appended version.
	latest recordChangeState
}

// Projection maintains shared mutable embedded indexes for timeline and analysis reads.
type Projection struct {
	mu sync.RWMutex

	retainHistoricalEventArrays bool
	retentionWindowNs           int64

	// Legacy history fields are kept only for compatibility with older helpers/tests.
	// Compact projection state should leave them empty after build/import/apply.
	events                    []models.Event
	eventsByResourceUID       map[string][]models.Event
	resourceMetaByUID         map[string]models.ResourceMetadata
	resourcesByUID            map[string]*resourceRecord
	resourcesByKey            map[resourceKey][]*resourceRecord
	k8sRawEventsByInvolvedUID map[string][]models.Event
	k8sEventsByInvolvedUID    map[string][]analysisstore.K8sEventInfo
	// k8sEventsTruncatedThrough records, per involved UID whose list hit the cap, the
	// newest timestamp of an event info that was dropped. The projection only has the
	// complete set of infos for windows starting after it.
	k8sEventsTruncatedThrough map[string]int64
	orderedResources          *orderedResourceIndex
	activeOrderedResources    []orderedResourceKey
	activeResourceKeyByUID    map[string]orderedResourceKey
	recentResourceChanges     []recentResourceChange
	minTimestampNs            int64
	maxTimestampNs            int64

	dataRoot       string
	segmentHandles map[string]*segmentDataHandle
}

type ProjectionSnapshot struct {
	Events                 []models.Event                          `json:"events,omitempty"`
	Resources              []ProjectionResourceSnapshot            `json:"resources,omitempty"`
	K8sEventsByInvolvedUID map[string][]analysisstore.K8sEventInfo `json:"k8s_events_by_involved_uid,omitempty"`
	MinTimestampNs         int64                                   `json:"min_timestamp_ns"`
	MaxTimestampNs         int64                                   `json:"max_timestamp_ns"`
}

type ProjectionResourceSnapshot struct {
	UID      string                              `json:"uid"`
	Versions []ProjectionResourceVersionSnapshot `json:"versions"`
}

type ProjectionResourceVersionSnapshot struct {
	EventID     string                        `json:"event_id"`
	Timestamp   int64                         `json:"timestamp"`
	EventType   models.EventType              `json:"event_type"`
	Identity    graph.ResourceIdentity        `json:"identity"`
	Data        []byte                        `json:"data,omitempty"`
	ChangeEvent analysisstore.ChangeEventInfo `json:"change_event"`
	// DataSegment is a 1-based index into the checkpoint's segment table; together with
	// DataOffset/DataLength it locates a payload that is not stored inline.
	DataSegment uint32 `json:"data_segment,omitempty"`
	DataOffset  int64  `json:"data_offset,omitempty"`
	DataLength  uint32 `json:"data_length,omitempty"`
	DataHash    uint64 `json:"data_hash,omitempty"`
}

func NewProjection() *Projection {
	return &Projection{
		eventsByResourceUID:       make(map[string][]models.Event),
		resourceMetaByUID:         make(map[string]models.ResourceMetadata),
		resourcesByUID:            make(map[string]*resourceRecord),
		resourcesByKey:            make(map[resourceKey][]*resourceRecord),
		orderedResources:          newOrderedResourceIndex(),
		k8sRawEventsByInvolvedUID: make(map[string][]models.Event),
		k8sEventsByInvolvedUID:    make(map[string][]analysisstore.K8sEventInfo),
		k8sEventsTruncatedThrough: make(map[string]int64),
		activeResourceKeyByUID:    make(map[string]orderedResourceKey),
		minTimestampNs:            -1,
		maxTimestampNs:            -1,
	}
}

func (p *Projection) EnableHistoricalEventRetention() {
	if p == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.retainHistoricalEventArrays = true
}

func (p *Projection) ResourceCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.resourcesByUID)
}
