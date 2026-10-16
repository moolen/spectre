package embeddedstore

import (
	"iter"
	"slices"
	"sort"
)

// graphFacts holds the fields of one object state that the analysis store derives graph
// edges from: owner references, name-based references, the pod selector and
// spec.replicas. They are extracted once at ingest from the already parsed payload and
// shared through versionMeta, so edge queries never read payloads back from segments.
// Values are immutable once a versionMeta references them, except that the projection
// may swap shape for an equal one under its write lock.
type graphFacts struct {
	// shape holds everything but the node; pods of one ReplicaSet share it (see
	// Projection.shareShapeLocked).
	shape *graphShape
	// node is a pod's spec.nodeName.
	node string
}

type graphShape struct {
	owners []string
	// refs are the name-based references other than SCHEDULED_ON; the node reference
	// belongs at index nodeAt.
	refs     []directRef
	nodeAt   int
	selector *podSelector
	// replicas is spec.replicas when hasReplicas is set.
	replicas    int64
	hasReplicas bool
}

var (
	emptyGraphShape = &graphShape{}
	// emptyGraphFacts describes an object without any edge-relevant fields. Versions
	// without a payload (most deletes) report it, matching edges derived from an empty
	// object.
	emptyGraphFacts = &graphFacts{shape: emptyGraphShape}
)

// podSelector is a parsed label selector. It keeps the matching rules of the JSON form
// it replaces: a selector without matchLabels uses its whole map as match labels, and
// any non-string match label value never matches.
type podSelector struct {
	matchLabels []labelPair
	exprs       []selectorExpr
	invalid     bool
}

type labelPair struct {
	key   string
	value string
}

type selectorExpr struct {
	key      string
	operator string
	values   []string
}

func graphFactsFromObject(kind, namespace string, object map[string]any) *graphFacts {
	if len(object) == 0 {
		return emptyGraphFacts
	}
	facts := &graphFacts{shape: &graphShape{}}
	shape := facts.shape
	for _, owner := range ownerUIDs(object) {
		shape.owners = append(shape.owners, internString(owner))
	}
	for _, ref := range directReferencesFromObject(kind, namespace, object) {
		if ref.relType == "SCHEDULED_ON" && facts.node == "" {
			facts.node = internString(ref.name)
			shape.nodeAt = len(shape.refs)
			continue
		}
		shape.refs = append(shape.refs, directRef{
			kind:      internString(ref.kind),
			name:      internString(ref.name),
			namespace: internString(ref.namespace),
			relType:   ref.relType,
		})
	}
	if facts.node == "" {
		shape.nodeAt = 0
	}
	if resourceUsesPodSelector(kind) {
		shape.selector = newPodSelector(selectorSpecFromObject(kind, object))
	}
	if replicas := specReplicas(object); replicas != nil {
		shape.replicas, shape.hasReplicas = int64(*replicas), true
	}
	if shape.equal(emptyGraphShape) {
		facts.shape = emptyGraphShape
		if facts.node == "" {
			return emptyGraphFacts
		}
	}
	return facts
}

func (f *graphFacts) equal(other *graphFacts) bool {
	if f == other {
		return true
	}
	if f == nil || other == nil {
		return false
	}
	return f.node == other.node && f.shape.equal(other.shape)
}

func (s *graphShape) equal(other *graphShape) bool {
	if s == other {
		return true
	}
	if s == nil || other == nil {
		return false
	}
	return slices.Equal(s.owners, other.owners) &&
		slices.Equal(s.refs, other.refs) &&
		s.nodeAt == other.nodeAt &&
		s.selector.equal(other.selector) &&
		s.replicas == other.replicas &&
		s.hasReplicas == other.hasReplicas
}

func (f *graphFacts) owners() []string {
	return f.shape.owners
}

func (f *graphFacts) selector() *podSelector {
	return f.shape.selector
}

// refs yields the name-based references in extraction order, the node a pod is
// scheduled on included.
func (f *graphFacts) refs() iter.Seq[directRef] {
	return func(yield func(directRef) bool) {
		for i, ref := range f.shape.refs {
			if f.node != "" && i == f.shape.nodeAt && !yield(f.nodeRef()) {
				return
			}
			if !yield(ref) {
				return
			}
		}
		if f.node != "" && f.shape.nodeAt >= len(f.shape.refs) {
			yield(f.nodeRef())
		}
	}
}

func (f *graphFacts) nodeRef() directRef {
	return directRef{kind: "Node", name: f.node, relType: "SCHEDULED_ON"}
}

func (f *graphFacts) specReplicas() *int {
	if f == nil || !f.shape.hasReplicas {
		return nil
	}
	replicas := int(f.shape.replicas)
	return &replicas
}

// factsFor returns the graph facts of a version. Versions restored from checkpoints
// written before facts were stored fall back to parsing their payload.
func factsFor(version *resourceVersion) *graphFacts {
	if version == nil || version.flags&versionFlagNoPayload != 0 || !versionHasPayload(version) {
		return emptyGraphFacts
	}
	if version.meta != nil && version.meta.graph != nil {
		return version.meta.graph
	}
	meta := version.metaOrEmpty()
	return graphFactsFromObject(meta.Kind, meta.Namespace, parsedVersionObject(version))
}

func newPodSelector(raw map[string]any) *podSelector {
	if len(raw) == 0 {
		return nil
	}
	selector := &podSelector{}
	matchLabels := getMap(raw, "matchLabels")
	if len(matchLabels) == 0 {
		matchLabels = raw
	}
	for key, value := range matchLabels {
		stringValue, ok := value.(string)
		if !ok {
			selector.invalid = true
			continue
		}
		selector.matchLabels = append(selector.matchLabels, labelPair{key: internString(key), value: internString(stringValue)})
	}
	sort.Slice(selector.matchLabels, func(i, j int) bool { return selector.matchLabels[i].key < selector.matchLabels[j].key })
	for _, item := range getSlice(raw, "matchExpressions") {
		expr, _ := item.(map[string]any)
		parsed := selectorExpr{key: internString(getString(expr, "key")), operator: getString(expr, "operator")}
		for _, value := range getSlice(expr, "values") {
			// Non-string values neither match In nor exclude for NotIn.
			if stringValue, ok := value.(string); ok {
				parsed.values = append(parsed.values, internString(stringValue))
			}
		}
		selector.exprs = append(selector.exprs, parsed)
	}
	return selector
}

func (s *podSelector) equal(other *podSelector) bool {
	if s == other {
		return true
	}
	if s == nil || other == nil {
		return false
	}
	if s.invalid != other.invalid || !slices.Equal(s.matchLabels, other.matchLabels) || len(s.exprs) != len(other.exprs) {
		return false
	}
	for i := range s.exprs {
		a, b := s.exprs[i], other.exprs[i]
		if a.key != b.key || a.operator != b.operator || !slices.Equal(a.values, b.values) {
			return false
		}
	}
	return true
}

// matches reports whether labels satisfy the selector. A nil selector matches nothing.
func (s *podSelector) matches(labels map[string]string) bool {
	if s == nil || s.invalid {
		return false
	}
	for _, pair := range s.matchLabels {
		// A missing label reads as "", as in the JSON form this replaces.
		if labels[pair.key] != pair.value {
			return false
		}
	}
	for _, expr := range s.exprs {
		labelValue, hasLabel := labels[expr.key]
		switch expr.operator {
		case "In":
			if !slices.Contains(expr.values, labelValue) {
				return false
			}
		case "NotIn":
			if slices.Contains(expr.values, labelValue) {
				return false
			}
		case "Exists":
			if !hasLabel {
				return false
			}
		case "DoesNotExist":
			if hasLabel {
				return false
			}
		}
	}
	return true
}
