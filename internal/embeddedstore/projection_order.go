package embeddedstore

import (
	"sort"

	"github.com/moolen/spectre/internal/models"
)

func insertEventSorted(events []models.Event, event models.Event) []models.Event {
	idx := sort.Search(len(events), func(i int) bool {
		return compareEventOrder(events[i], event) > 0
	})

	events = append(events, models.Event{})
	copy(events[idx+1:], events[idx:])
	events[idx] = event
	return events
}

func appendOrInsertEventSorted(events []models.Event, event models.Event) []models.Event {
	if len(events) == 0 || compareEventOrder(events[len(events)-1], event) <= 0 {
		return append(events, event)
	}
	return insertEventSorted(events, event)
}

func insertOrderedResourceKey(keys []orderedResourceKey, key orderedResourceKey) []orderedResourceKey {
	idx := sort.Search(len(keys), func(i int) bool {
		return compareOrderedResourceKey(keys[i], key) >= 0
	})
	if idx < len(keys) && compareOrderedResourceKey(keys[idx], key) == 0 {
		return keys
	}

	keys = append(keys, orderedResourceKey{})
	copy(keys[idx+1:], keys[idx:])
	keys[idx] = key
	return keys
}

func removeOrderedResourceKey(keys []orderedResourceKey, key orderedResourceKey) []orderedResourceKey {
	idx := sort.Search(len(keys), func(i int) bool {
		return compareOrderedResourceKey(keys[i], key) >= 0
	})
	if idx >= len(keys) || compareOrderedResourceKey(keys[idx], key) != 0 {
		return keys
	}

	copy(keys[idx:], keys[idx+1:])
	keys = keys[:len(keys)-1]
	return keys
}

func compareEventOrder(left, right models.Event) int {
	if left.Timestamp != right.Timestamp {
		if left.Timestamp < right.Timestamp {
			return -1
		}
		return 1
	}
	switch {
	case left.ID < right.ID:
		return -1
	case left.ID > right.ID:
		return 1
	default:
		return 0
	}
}

func compareOrderedResourceKey(left, right orderedResourceKey) int {
	if left.kind != right.kind {
		if left.kind < right.kind {
			return -1
		}
		return 1
	}
	if left.namespace != right.namespace {
		if left.namespace < right.namespace {
			return -1
		}
		return 1
	}
	if left.name != right.name {
		if left.name < right.name {
			return -1
		}
		return 1
	}
	if left.uid < right.uid {
		return -1
	}
	if left.uid > right.uid {
		return 1
	}
	return 0
}

func cloneEvent(event models.Event) models.Event {
	cloned := event
	cloned.Data = cloneBytes(event.Data)
	return cloned
}

func cloneEvents(events []models.Event) []models.Event {
	if len(events) == 0 {
		return nil
	}
	cloned := make([]models.Event, len(events))
	for i := range events {
		cloned[i] = cloneEvent(events[i])
	}
	return cloned
}

func cloneBytes(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}
	return append([]byte(nil), data...)
}
