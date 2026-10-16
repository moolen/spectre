package embeddedstore

// checkpointV3NewWithGraph marks a new identity block followed by its graph facts.
// Blocks written with checkpointV3New predate facts; their facts are derived from
// payloads after loading (see resolvePendingGraphFacts).
const checkpointV3NewWithGraph byte = 3

const (
	checkpointV3GraphSelector byte = 1 << iota
	checkpointV3GraphSelectorInvalid
	checkpointV3GraphReplicas
)

func (c *checkpointV3Writer) graphFacts(facts *graphFacts) {
	shape := facts.shape
	c.string(facts.node)
	c.uvarint(uint64(shape.nodeAt))
	c.stringList(shape.owners)
	c.uvarint(uint64(len(shape.refs)))
	for _, ref := range shape.refs {
		c.string(ref.kind)
		c.string(ref.name)
		c.string(ref.namespace)
		c.string(ref.relType)
	}
	var flags byte
	if shape.selector != nil {
		flags |= checkpointV3GraphSelector
		if shape.selector.invalid {
			flags |= checkpointV3GraphSelectorInvalid
		}
	}
	if shape.hasReplicas {
		flags |= checkpointV3GraphReplicas
	}
	c.byte(flags)
	if selector := shape.selector; selector != nil {
		c.uvarint(uint64(len(selector.matchLabels)))
		for _, pair := range selector.matchLabels {
			c.string(pair.key)
			c.string(pair.value)
		}
		c.uvarint(uint64(len(selector.exprs)))
		for _, expr := range selector.exprs {
			c.string(expr.key)
			c.string(expr.operator)
			c.stringList(expr.values)
		}
	}
	if shape.hasReplicas {
		c.varint(shape.replicas)
	}
}

// graphFacts decodes facts, reusing previous (or its shape) when equal so consecutive
// metas keep sharing one value.
func (c *checkpointV3Reader) graphFacts(previous *graphFacts) *graphFacts {
	facts := &graphFacts{node: internString(c.string()), shape: &graphShape{}}
	shape := facts.shape
	shape.nodeAt = int(min(c.uvarint(), 1<<16))
	for _, owner := range c.stringList() {
		shape.owners = append(shape.owners, internString(owner))
	}
	refCount := c.uvarint()
	for i := uint64(0); i < refCount && c.err == nil; i++ {
		shape.refs = append(shape.refs, directRef{
			kind:      internString(c.string()),
			name:      internString(c.string()),
			namespace: internString(c.string()),
			relType:   internString(c.string()),
		})
	}
	flags := c.byte()
	if flags&checkpointV3GraphSelector != 0 {
		selector := &podSelector{invalid: flags&checkpointV3GraphSelectorInvalid != 0}
		labelCount := c.uvarint()
		for i := uint64(0); i < labelCount && c.err == nil; i++ {
			selector.matchLabels = append(selector.matchLabels, labelPair{key: internString(c.string()), value: internString(c.string())})
		}
		exprCount := c.uvarint()
		for i := uint64(0); i < exprCount && c.err == nil; i++ {
			expr := selectorExpr{key: internString(c.string()), operator: c.string()}
			for _, value := range c.stringList() {
				expr.values = append(expr.values, internString(value))
			}
			selector.exprs = append(selector.exprs, expr)
		}
		shape.selector = selector
	}
	if flags&checkpointV3GraphReplicas != 0 {
		shape.replicas, shape.hasReplicas = c.varint(), true
	}
	switch {
	case facts.equal(previous):
		return previous
	case previous != nil && shape.equal(previous.shape):
		facts.shape = previous.shape
	case shape.equal(emptyGraphShape):
		facts.shape = emptyGraphShape
		if facts.node == "" {
			return emptyGraphFacts
		}
	}
	return facts
}
