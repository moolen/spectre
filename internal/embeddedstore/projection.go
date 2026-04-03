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

type resourceVersion struct {
	event       models.Event
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

// Projection maintains shared mutable embedded indexes for timeline and analysis reads.
type Projection struct {
	mu sync.RWMutex

	events                    []models.Event
	eventsByResourceUID       map[string][]models.Event
	resourceMetaByUID         map[string]models.ResourceMetadata
	resourcesByUID            map[string]*resourceRecord
	resourcesByKey            map[resourceKey][]*resourceRecord
	k8sRawEventsByInvolvedUID map[string][]models.Event
	k8sEventsByInvolvedUID    map[string][]analysisstore.K8sEventInfo
	orderedResources          []orderedResourceKey
	minTimestampNs            int64
	maxTimestampNs            int64
}

type ProjectionSnapshot struct {
	Events []models.Event `json:"events"`
}

func NewProjection() *Projection {
	return &Projection{
		eventsByResourceUID:       make(map[string][]models.Event),
		resourceMetaByUID:         make(map[string]models.ResourceMetadata),
		resourcesByUID:            make(map[string]*resourceRecord),
		resourcesByKey:            make(map[resourceKey][]*resourceRecord),
		k8sRawEventsByInvolvedUID: make(map[string][]models.Event),
		k8sEventsByInvolvedUID:    make(map[string][]analysisstore.K8sEventInfo),
		minTimestampNs:            -1,
		maxTimestampNs:            -1,
	}
}
