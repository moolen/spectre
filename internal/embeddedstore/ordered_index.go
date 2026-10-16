package embeddedstore

import (
	"github.com/google/btree"
	"github.com/moolen/spectre/internal/models"
)

// orderedResourceIndex keeps resource keys in (kind, namespace, name, uid) order. It is a
// B-tree so inserts stay O(log n) with millions of resources, and pagination can seek
// straight to a cursor.
type orderedResourceIndex struct {
	tree *btree.BTreeG[orderedResourceKey]
}

func newOrderedResourceIndex() *orderedResourceIndex {
	return &orderedResourceIndex{tree: btree.NewG(32, func(a, b orderedResourceKey) bool {
		return compareOrderedResourceKey(a, b) < 0
	})}
}

func (x *orderedResourceIndex) Len() int {
	if x == nil {
		return 0
	}
	return x.tree.Len()
}

func (x *orderedResourceIndex) Insert(key orderedResourceKey) {
	x.tree.ReplaceOrInsert(key)
}

func (x *orderedResourceIndex) Delete(key orderedResourceKey) {
	x.tree.Delete(key)
}

// Ascend visits keys in order until fn returns false.
func (x *orderedResourceIndex) Ascend(fn func(orderedResourceKey) bool) {
	if x == nil {
		return
	}
	x.tree.Ascend(fn)
}

// AscendAfterCursor visits keys ordered after the cursor's (kind, namespace, name).
func (x *orderedResourceIndex) AscendAfterCursor(cursor *models.ResourceCursor, fn func(orderedResourceKey) bool) {
	if x == nil {
		return
	}
	if cursor == nil {
		x.tree.Ascend(fn)
		return
	}
	pivot := orderedResourceKey{kind: cursor.Kind, namespace: cursor.Namespace, name: cursor.Name}
	x.tree.AscendGreaterOrEqual(pivot, func(key orderedResourceKey) bool {
		if compareOrderedResourceKeyToCursor(key, cursor) <= 0 {
			return true
		}
		return fn(key)
	})
}

// Keys returns a copy of all keys in order.
func (x *orderedResourceIndex) Keys() []orderedResourceKey {
	keys := make([]orderedResourceKey, 0, x.Len())
	x.Ascend(func(key orderedResourceKey) bool {
		keys = append(keys, key)
		return true
	})
	return keys
}
