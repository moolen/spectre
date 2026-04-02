package embeddedstore

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/moolen/spectre/internal/models"
)

func TestJournal_AppendReplayOrder(t *testing.T) {
	t.Parallel()

	journal := openTestJournal(t)
	ctx := context.Background()

	events := []models.Event{
		newTestEvent("event-1", 100),
		newTestEvent("event-2", 200),
		newTestEvent("event-3", 300),
	}

	if err := journal.AppendBatch(ctx, events); err != nil {
		t.Fatalf("append batch: %v", err)
	}

	replayed, err := journal.Replay(ctx)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}

	if len(replayed) != len(events) {
		t.Fatalf("expected %d events, got %d", len(events), len(replayed))
	}

	for i := range events {
		if replayed[i].ID != events[i].ID {
			t.Fatalf("event %d id mismatch: got %q want %q", i, replayed[i].ID, events[i].ID)
		}
		if replayed[i].Timestamp != events[i].Timestamp {
			t.Fatalf("event %d timestamp mismatch: got %d want %d", i, replayed[i].Timestamp, events[i].Timestamp)
		}
	}
}

func TestJournal_ReopenThenReplay(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ctx := context.Background()

	journal, err := OpenJournal(root)
	if err != nil {
		t.Fatalf("open journal: %v", err)
	}
	t.Cleanup(func() {
		_ = journal.Close()
	})

	events := []models.Event{
		newTestEvent("event-a", 1000),
		newTestEvent("event-b", 2000),
	}

	if err := journal.AppendBatch(ctx, events); err != nil {
		t.Fatalf("append batch: %v", err)
	}

	if err := journal.Close(); err != nil {
		t.Fatalf("close journal: %v", err)
	}

	reopened, err := OpenJournal(root)
	if err != nil {
		t.Fatalf("re-open journal: %v", err)
	}
	t.Cleanup(func() {
		_ = reopened.Close()
	})

	replayed, err := reopened.Replay(ctx)
	if err != nil {
		t.Fatalf("replay from reopened journal: %v", err)
	}

	if len(replayed) != len(events) {
		t.Fatalf("expected %d events, got %d", len(events), len(replayed))
	}
	for i := range events {
		if replayed[i].ID != events[i].ID {
			t.Fatalf("event %d id mismatch: got %q want %q", i, replayed[i].ID, events[i].ID)
		}
	}
}

func TestJournal_AppendBatchEmptyNoop(t *testing.T) {
	t.Parallel()

	journal := openTestJournal(t)
	ctx := context.Background()

	if err := journal.AppendBatch(ctx, nil); err != nil {
		t.Fatalf("append nil batch: %v", err)
	}
	if err := journal.AppendBatch(ctx, []models.Event{}); err != nil {
		t.Fatalf("append empty batch: %v", err)
	}

	info, err := os.Stat(journal.path)
	if err != nil {
		t.Fatalf("stat journal: %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("expected empty journal file, got size %d", info.Size())
	}
}

func TestJournal_ReplayCorruptOrTruncatedReturnsError(t *testing.T) {
	t.Parallel()

	t.Run("truncated", func(t *testing.T) {
		t.Parallel()

		journal := openTestJournal(t)
		ctx := context.Background()

		if err := journal.Append(ctx, newTestEvent("ok", 1)); err != nil {
			t.Fatalf("append: %v", err)
		}

		corruptFile, err := os.OpenFile(journal.path, os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			t.Fatalf("open for corruption: %v", err)
		}
		t.Cleanup(func() {
			_ = corruptFile.Close()
		})

		var header [4]byte
		binary.BigEndian.PutUint32(header[:], 16)
		if _, err := corruptFile.Write(header[:]); err != nil {
			t.Fatalf("write corrupt header: %v", err)
		}
		if _, err := corruptFile.Write([]byte(`{"id":"oops"`)); err != nil {
			t.Fatalf("write truncated payload: %v", err)
		}
		if err := corruptFile.Sync(); err != nil {
			t.Fatalf("sync corrupt write: %v", err)
		}

		_, err = journal.Replay(ctx)
		if err == nil {
			t.Fatal("expected replay error for truncated journal entry")
		}
		if !strings.Contains(err.Error(), "truncated") {
			t.Fatalf("expected truncated error, got: %v", err)
		}
	})

	t.Run("corrupt", func(t *testing.T) {
		t.Parallel()

		journal := openTestJournal(t)
		ctx := context.Background()

		if err := journal.Append(ctx, newTestEvent("ok", 1)); err != nil {
			t.Fatalf("append: %v", err)
		}

		corruptFile, err := os.OpenFile(journal.path, os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			t.Fatalf("open for corruption: %v", err)
		}
		t.Cleanup(func() {
			_ = corruptFile.Close()
		})

		payload := []byte("not-json")
		var header [4]byte
		binary.BigEndian.PutUint32(header[:], uint32(len(payload)))
		if _, err := corruptFile.Write(header[:]); err != nil {
			t.Fatalf("write corrupt header: %v", err)
		}
		if _, err := corruptFile.Write(payload); err != nil {
			t.Fatalf("write corrupt payload: %v", err)
		}
		if err := corruptFile.Sync(); err != nil {
			t.Fatalf("sync corrupt write: %v", err)
		}

		_, err = journal.Replay(ctx)
		if err == nil {
			t.Fatal("expected replay error for corrupt journal entry")
		}
		if !strings.Contains(err.Error(), "corrupt") {
			t.Fatalf("expected corrupt error, got: %v", err)
		}
	})
}

func TestJournal_AppendBatchOversizedPayloadReturnsError(t *testing.T) {
	t.Parallel()

	journal := openTestJournal(t)
	ctx := context.Background()

	oversizedData := `"` + strings.Repeat("a", maxJournalRecordSize+1) + `"`
	event := newTestEvent("too-big", 123)
	event.Data = []byte(oversizedData)

	err := journal.AppendBatch(ctx, []models.Event{event})
	if err == nil {
		t.Fatal("expected append to fail for oversized record")
	}
	if !strings.Contains(err.Error(), "max") {
		t.Fatalf("expected size limit error, got: %v", err)
	}
}

func TestJournal_ReplayOversizedHeaderReturnsError(t *testing.T) {
	t.Parallel()

	journal := openTestJournal(t)
	ctx := context.Background()

	corruptFile, err := os.OpenFile(journal.path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatalf("open for corruption: %v", err)
	}
	t.Cleanup(func() {
		_ = corruptFile.Close()
	})

	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(maxJournalRecordSize+1))
	if _, err := corruptFile.Write(header[:]); err != nil {
		t.Fatalf("write oversized header: %v", err)
	}
	if err := corruptFile.Sync(); err != nil {
		t.Fatalf("sync corrupt write: %v", err)
	}

	_, err = journal.Replay(ctx)
	if err == nil {
		t.Fatal("expected replay error for oversized record header")
	}
	if !strings.Contains(err.Error(), "oversized") {
		t.Fatalf("expected oversized error, got: %v", err)
	}
	if !strings.Contains(err.Error(), strconv.Itoa(maxJournalRecordSize+1)) {
		t.Fatalf("expected header size in error, got: %v", err)
	}
}

func openTestJournal(t *testing.T) *Journal {
	t.Helper()

	root := t.TempDir()
	journal, err := OpenJournal(root)
	if err != nil {
		t.Fatalf("open journal: %v", err)
	}
	t.Cleanup(func() {
		_ = journal.Close()
	})

	return journal
}

func newTestEvent(id string, ts int64) models.Event {
	return models.Event{
		ID:        id,
		Timestamp: ts,
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Version:   "v1",
			Kind:      "Pod",
			Namespace: "default",
			Name:      id,
			UID:       id + "-uid",
		},
		Data: []byte(`{"kind":"Pod"}`),
	}
}

func TestJournal_FileLocationUnderRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	journal, err := OpenJournal(root)
	if err != nil {
		t.Fatalf("open journal: %v", err)
	}
	t.Cleanup(func() {
		_ = journal.Close()
	})

	if filepath.Dir(journal.path) != root {
		t.Fatalf("journal file dir mismatch: got %q want %q", filepath.Dir(journal.path), root)
	}
}
