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

type resourceVersion struct {
	eventID     string
	timestamp   int64
	eventType   models.EventType
	identity    graph.ResourceIdentity
	data        []byte
	changeEvent analysisstore.ChangeEventInfo
}

type resourceRecord struct {
	uid      string
	versions []resourceVersion
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
	orderedResources          []orderedResourceKey
	activeOrderedResources    []orderedResourceKey
	activeResourceKeyByUID    map[string]orderedResourceKey
	recentResourceChanges     []recentResourceChange
	minTimestampNs            int64
	maxTimestampNs            int64
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
}

func NewProjection() *Projection {
	return &Projection{
		eventsByResourceUID:       make(map[string][]models.Event),
		resourceMetaByUID:         make(map[string]models.ResourceMetadata),
		resourcesByUID:            make(map[string]*resourceRecord),
		resourcesByKey:            make(map[resourceKey][]*resourceRecord),
		k8sRawEventsByInvolvedUID: make(map[string][]models.Event),
		k8sEventsByInvolvedUID:    make(map[string][]analysisstore.K8sEventInfo),
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
