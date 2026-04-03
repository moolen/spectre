package falkor

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/graph"
)

const (
	queryTimeoutMs      = 120000
	maxRecentEvents     = 10
	maxK8sEvents        = 20
	defaultLimit        = 50
	maxLimit            = 500
	defaultMaxDepth     = 1
	maxMaxDepth         = 10
	defaultLookbackNs   = int64(30 * time.Minute)
	maxLookbackNs       = int64(24 * time.Hour)
	statusUnknown       = "Unknown"
	edgeTypeIngressRef  = "INGRESS_REF"
	edgeTypeReferences  = "REFERENCES_SPEC"
	defaultManagerFloor = 0.5
)

type Store struct {
	graphClient graph.Client
}

var _ store.AnalysisStore = (*Store)(nil)

func New(graphClient graph.Client) *Store {
	return &Store{graphClient: graphClient}
}

type resourceResult struct {
	UID       string
	Kind      string
	APIGroup  string
	Namespace string
	Name      string
	Labels    map[string]string
	FirstSeen int64
	LastSeen  int64
	Deleted   bool
	DeletedAt int64
}

type edgeResult struct {
	SourceUID        string
	TargetUID        string
	RelationshipType string
	EdgeID           string
}

type paginationCursor struct {
	LastKind string `json:"lastKind"`
	LastName string `json:"lastName"`
}

type specChangeResult struct {
	ResourceUID     string
	LatestData      []byte
	EarliestData    []byte
	LatestTimestamp int64
}

func encodeCursor(cursor paginationCursor) string {
	data, _ := json.Marshal(cursor)
	return base64.StdEncoding.EncodeToString(data)
}

func decodeCursor(cursor string) (*paginationCursor, error) {
	data, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return nil, fmt.Errorf("failed to decode cursor: %w", err)
	}
	var pc paginationCursor
	if err := json.Unmarshal(data, &pc); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cursor: %w", err)
	}
	return &pc, nil
}
