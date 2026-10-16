package embeddedstore

import (
	"runtime"
	"sync"
)

// Reverse indexes over graph facts. They let the analysis store find the resources in a
// namespace and the resources pointing at a given one without scanning every record.
// Entries are added for every set of facts a record had; they are candidates that
// queries check against the version visible at the query time, so stale entries (facts
// that changed since) cost a lookup but never produce an edge.
type graphIndexes struct {
	byNamespace map[string][]*resourceRecord
	// byOwner maps an owner UID to the records whose ownerReferences name it.
	byOwner map[string][]*resourceRecord
	// byTarget maps a (namespace, kind, name) key to the records whose name-based
	// references or manager labels resolve to it.
	byTarget map[resourceKey][]*resourceRecord
	// pending holds records restored with metas that have no facts yet.
	pending []*resourceRecord
}

func newGraphIndexes() graphIndexes {
	return graphIndexes{
		byNamespace: make(map[string][]*resourceRecord),
		byOwner:     make(map[string][]*resourceRecord),
		byTarget:    make(map[resourceKey][]*resourceRecord),
	}
}

// graphKeys are the index keys one versionMeta contributes.
type graphKeys struct {
	owners  []string
	targets []resourceKey
}

func metaGraphKeys(meta *versionMeta) graphKeys {
	var keys graphKeys
	if meta == nil {
		return keys
	}
	if facts := meta.graph; facts != nil {
		keys.owners = facts.owners()
		for ref := range facts.refs() {
			keys.targets = append(keys.targets, resourceKey{namespace: ref.namespaceFor(meta.Namespace), kind: ref.kind, name: ref.name})
		}
	}
	if manager := managerReferenceFor(meta.Labels, meta.Namespace); manager != nil {
		keys.targets = append(keys.targets, resourceKey{namespace: manager.namespace, kind: manager.kind, name: manager.name})
		if manager.namespace == "" {
			keys.targets = append(keys.targets, resourceKey{namespace: meta.Namespace, kind: manager.kind, name: manager.name})
		}
	}
	return keys
}

func appendRecordOnce(records []*resourceRecord, record *resourceRecord) []*resourceRecord {
	if n := len(records); n > 0 && records[n-1] == record {
		return records
	}
	return append(records, record)
}

func (p *Projection) indexRecordNamespaceLocked(record *resourceRecord, namespace string) {
	namespace = internString(namespace)
	p.graph.byNamespace[namespace] = appendRecordOnce(p.graph.byNamespace[namespace], record)
}

// indexMetaLocked adds record under the keys of meta that previous (the meta of the
// version before it, or nil) did not already contribute.
func (p *Projection) indexMetaLocked(record *resourceRecord, meta, previous *versionMeta) {
	if meta == nil || meta == previous {
		return
	}
	if meta.graph == nil && meta.Labels == nil {
		return
	}
	if previous != nil && previous.graph == meta.graph && mapIdentity(previous.Labels) == mapIdentity(meta.Labels) {
		return
	}
	if previous == nil || previous.graph != meta.graph {
		p.shareShapeLocked(record, meta.graph)
	}
	keys := metaGraphKeys(meta)
	old := metaGraphKeys(previous)
	for _, owner := range keys.owners {
		if !containsString(old.owners, owner) {
			p.graph.byOwner[owner] = appendRecordOnce(p.graph.byOwner[owner], record)
		}
	}
	for _, target := range keys.targets {
		if !containsKey(old.targets, target) {
			p.graph.byTarget[target] = appendRecordOnce(p.graph.byTarget[target], record)
		}
	}
}

// shareShapeLocked points facts at an equal shape of a sibling (a resource with the
// same owner), so the pods of one ReplicaSet hold one copy of their owner, references
// and selector. Only the shape pointer changes, under the projection's write lock.
func (p *Projection) shareShapeLocked(record *resourceRecord, facts *graphFacts) {
	if facts == nil || len(facts.shape.owners) == 0 {
		return
	}
	siblings := p.graph.byOwner[facts.shape.owners[0]]
	for i := len(siblings) - 1; i >= 0 && i >= len(siblings)-3; i-- {
		sibling := siblings[i]
		if sibling == record {
			continue
		}
		latest := sibling.latestVersion()
		if latest == nil || latest.meta == nil || latest.meta.graph == nil {
			continue
		}
		if shape := latest.meta.graph.shape; shape != facts.shape && shape.equal(facts.shape) {
			facts.shape = shape
			return
		}
	}
}

// indexRecordLocked indexes every meta of a restored record. Records with metas that
// predate stored facts are queued for resolvePendingGraphFacts.
func (p *Projection) indexRecordLocked(record *resourceRecord) {
	if len(record.versions) == 0 {
		return
	}
	p.indexRecordNamespaceLocked(record, record.versions[len(record.versions)-1].metaOrEmpty().Namespace)
	var previous *versionMeta
	queued := false
	for i := range record.versions {
		version := &record.versions[i]
		meta := version.meta
		if meta != nil && meta.graph == nil && versionHasPayload(version) && !queued {
			p.graph.pending = append(p.graph.pending, record)
			queued = true
		}
		p.indexMetaLocked(record, meta, previous)
		previous = meta
	}
}

// resolvePendingGraphFacts derives facts for metas restored without them by reading the
// newest payload of each such meta, then indexes them. Consecutive versions shared a
// meta when their identity and labels matched, so this is exact unless edge fields
// changed under unchanged labels (a pod getting scheduled); such history ages out with
// retention. It runs once after loading an older checkpoint.
func (p *Projection) resolvePendingGraphFacts() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.graph.pending) == 0 {
		return
	}
	type job struct {
		meta    *versionMeta
		version *resourceVersion
	}
	var jobs []job
	seen := make(map[*versionMeta]bool)
	for _, record := range p.graph.pending {
		// Prefer, per meta, the newest version known to carry an object: empty delete
		// records would yield no facts for the whole meta.
		for pass := 0; pass < 2; pass++ {
			for i := len(record.versions) - 1; i >= 0; i-- {
				version := &record.versions[i]
				meta := version.meta
				if meta == nil || meta.graph != nil || seen[meta] || !versionHasPayload(version) {
					continue
				}
				if pass == 0 && (version.dataHash == 0 || version.flags&versionFlagNoPayload != 0) {
					continue
				}
				seen[meta] = true
				jobs = append(jobs, job{meta: meta, version: version})
			}
		}
	}
	results := make([]*graphFacts, len(jobs))
	var wg sync.WaitGroup
	workers := max(1, runtime.GOMAXPROCS(0))
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := w; i < len(jobs); i += workers {
				meta := jobs[i].meta
				results[i] = graphFactsFromObject(meta.Kind, meta.Namespace, parsedVersionObject(jobs[i].version))
			}
		}(w)
	}
	wg.Wait()
	for i := range jobs {
		jobs[i].meta.graph = results[i]
	}
	for _, record := range p.graph.pending {
		var previous *versionMeta
		for i := range record.versions {
			p.indexMetaLocked(record, record.versions[i].meta, previous)
			previous = record.versions[i].meta
		}
	}
	p.graph.pending = nil
}

// unindexRecordsLocked removes records dropped by retention, plus index entries of
// metas that retention trimmed from records it kept (stale maps record -> its metas
// that are gone).
func (p *Projection) unindexRecordsLocked(removed map[*resourceRecord]bool, stale map[*resourceRecord][]*versionMeta) {
	if len(removed) == 0 && len(stale) == 0 {
		return
	}
	namespaces := make(map[string]bool)
	owners := make(map[string]bool)
	targets := make(map[resourceKey]bool)
	collect := func(meta *versionMeta) {
		if meta == nil {
			return
		}
		keys := metaGraphKeys(meta)
		for _, owner := range keys.owners {
			owners[owner] = true
		}
		for _, target := range keys.targets {
			targets[target] = true
		}
	}
	for record := range removed {
		seen := make(map[*versionMeta]bool)
		for i := range record.versions {
			meta := record.versions[i].meta
			if meta != nil && !seen[meta] {
				seen[meta] = true
				namespaces[meta.Namespace] = true
				collect(meta)
			}
		}
	}
	// A trimmed meta's keys are dropped only when no retained meta still has them.
	type staleEntry struct {
		record *resourceRecord
		owner  string
		target resourceKey
	}
	staleOwners := make(map[staleEntry]bool)
	staleTargets := make(map[staleEntry]bool)
	for record, metas := range stale {
		retained := make(map[*versionMeta]bool)
		var keep graphKeys
		for i := range record.versions {
			meta := record.versions[i].meta
			if meta != nil && !retained[meta] {
				retained[meta] = true
				keys := metaGraphKeys(meta)
				keep.owners = append(keep.owners, keys.owners...)
				keep.targets = append(keep.targets, keys.targets...)
			}
		}
		for _, meta := range metas {
			keys := metaGraphKeys(meta)
			for _, owner := range keys.owners {
				if !containsString(keep.owners, owner) {
					owners[owner] = true
					staleOwners[staleEntry{record: record, owner: owner}] = true
				}
			}
			for _, target := range keys.targets {
				if !containsKey(keep.targets, target) {
					targets[target] = true
					staleTargets[staleEntry{record: record, target: target}] = true
				}
			}
		}
	}

	filter := func(records []*resourceRecord, drop func(*resourceRecord) bool) []*resourceRecord {
		kept := records[:0]
		for _, record := range records {
			if !drop(record) {
				kept = append(kept, record)
			}
		}
		clear(records[len(kept):])
		return kept
	}
	for namespace := range namespaces {
		if kept := filter(p.graph.byNamespace[namespace], func(r *resourceRecord) bool { return removed[r] }); len(kept) > 0 {
			p.graph.byNamespace[namespace] = kept
		} else {
			delete(p.graph.byNamespace, namespace)
		}
	}
	for owner := range owners {
		kept := filter(p.graph.byOwner[owner], func(r *resourceRecord) bool {
			return removed[r] || staleOwners[staleEntry{record: r, owner: owner}]
		})
		if len(kept) > 0 {
			p.graph.byOwner[owner] = kept
		} else {
			delete(p.graph.byOwner, owner)
		}
	}
	for target := range targets {
		kept := filter(p.graph.byTarget[target], func(r *resourceRecord) bool {
			return removed[r] || staleTargets[staleEntry{record: r, target: target}]
		})
		if len(kept) > 0 {
			p.graph.byTarget[target] = kept
		} else {
			delete(p.graph.byTarget, target)
		}
	}
}

func containsKey(keys []resourceKey, key resourceKey) bool {
	for _, candidate := range keys {
		if candidate == key {
			return true
		}
	}
	return false
}
