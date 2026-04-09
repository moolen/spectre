package embeddedstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/moolen/spectre/internal/models"
)

const tailDirName = "tail"

type tailJournal struct {
	root    string
	meta    TailJournalMeta
	journal *Journal
}

type tailJournalRecord struct {
	HighWaterMark uint64       `json:"high_water_mark"`
	Event         models.Event `json:"event"`
}

func openTailJournal(root string, meta TailJournalMeta) (*tailJournal, error) {
	if root == "" {
		return nil, fmt.Errorf("open tail journal: root path is empty")
	}
	if meta.BaseHighWaterMark > meta.LastHighWaterMark {
		meta.LastHighWaterMark = meta.BaseHighWaterMark
	}
	if meta.ID == "" {
		meta.ID = newTailJournalID(meta.BaseHighWaterMark)
	}

	journal, err := OpenJournal(filepath.Join(root, tailDirName, meta.ID))
	if err != nil {
		return nil, fmt.Errorf("open tail journal %q: %w", meta.ID, err)
	}

	sizeBytes, err := journal.sizeBytes()
	if err != nil {
		_ = journal.Close()
		return nil, fmt.Errorf("open tail journal %q: stat size: %w", meta.ID, err)
	}
	meta.SizeBytes = sizeBytes

	return &tailJournal{
		root:    root,
		meta:    meta,
		journal: journal,
	}, nil
}

func (t *tailJournal) Close() error {
	if t == nil || t.journal == nil {
		return nil
	}

	return t.journal.Close()
}

func (t *tailJournal) AppendBatch(ctx context.Context, nextHighWaterMark uint64, events []models.Event) (TailJournalMeta, error) {
	if t == nil || t.journal == nil {
		return TailJournalMeta{}, fmt.Errorf("append tail journal batch: tail journal is nil")
	}
	if len(events) == 0 {
		return t.meta, nil
	}

	expectedHighWaterMark := t.meta.LastHighWaterMark + 1
	if nextHighWaterMark != expectedHighWaterMark {
		return TailJournalMeta{}, fmt.Errorf(
			"append tail journal batch: expected starting high-water mark %d, got %d",
			expectedHighWaterMark,
			nextHighWaterMark,
		)
	}

	payloads := make([][]byte, 0, len(events))
	for i := range events {
		record := tailJournalRecord{
			HighWaterMark: nextHighWaterMark + uint64(i),
			Event:         cloneEvent(events[i]),
		}
		payload, err := json.Marshal(record)
		if err != nil {
			return TailJournalMeta{}, fmt.Errorf("append tail journal batch: marshal record %d: %w", i, err)
		}
		payloads = append(payloads, payload)
	}

	if err := t.journal.appendPayloadBatch(ctx, payloads); err != nil {
		return TailJournalMeta{}, fmt.Errorf("append tail journal batch: %w", err)
	}

	sizeBytes, err := t.journal.sizeBytes()
	if err != nil {
		return TailJournalMeta{}, fmt.Errorf("append tail journal batch: stat size: %w", err)
	}

	t.meta.LastHighWaterMark = nextHighWaterMark + uint64(len(events)) - 1
	t.meta.EventCount += len(events)
	t.meta.SizeBytes = sizeBytes

	return t.meta, nil
}

func (t *tailJournal) ReplaySince(ctx context.Context, afterHighWaterMark uint64, apply func(models.Event, uint64) error) error {
	if t == nil || t.journal == nil {
		return fmt.Errorf("replay tail journal: tail journal is nil")
	}

	expectedHighWaterMark := t.meta.BaseHighWaterMark + 1
	lastHighWaterMark := t.meta.BaseHighWaterMark
	eventCount := 0
	if err := t.journal.replayPayloads(ctx, func(payload []byte) error {
		var record tailJournalRecord
		if err := json.Unmarshal(payload, &record); err != nil {
			return fmt.Errorf("corrupt journal entry: %w", err)
		}
		if record.HighWaterMark != expectedHighWaterMark {
			return fmt.Errorf("tail journal high-water mark gap: expected %d, got %d", expectedHighWaterMark, record.HighWaterMark)
		}
		expectedHighWaterMark++
		lastHighWaterMark = record.HighWaterMark
		eventCount++

		if record.HighWaterMark <= afterHighWaterMark || apply == nil {
			return nil
		}
		return apply(cloneEvent(record.Event), record.HighWaterMark)
	}); err != nil {
		return err
	}

	sizeBytes, err := t.journal.sizeBytes()
	if err != nil {
		return fmt.Errorf("replay tail journal: stat size: %w", err)
	}

	t.meta.LastHighWaterMark = lastHighWaterMark
	t.meta.EventCount = eventCount
	t.meta.SizeBytes = sizeBytes
	return nil
}

func (t *tailJournal) Rotate(newBaseHighWaterMark uint64) (TailJournalMeta, error) {
	if t == nil || t.journal == nil {
		return TailJournalMeta{}, fmt.Errorf("rotate tail journal: tail journal is nil")
	}
	if newBaseHighWaterMark < t.meta.LastHighWaterMark {
		return TailJournalMeta{}, fmt.Errorf(
			"rotate tail journal: new base high-water mark %d is behind last high-water mark %d",
			newBaseHighWaterMark,
			t.meta.LastHighWaterMark,
		)
	}

	next, err := openTailJournal(t.root, TailJournalMeta{
		ID:                newTailJournalID(newBaseHighWaterMark),
		BaseHighWaterMark: newBaseHighWaterMark,
		LastHighWaterMark: newBaseHighWaterMark,
	})
	if err != nil {
		return TailJournalMeta{}, err
	}

	if err := t.journal.Close(); err != nil {
		_ = next.Close()
		return TailJournalMeta{}, fmt.Errorf("rotate tail journal: close previous journal: %w", err)
	}

	t.meta = next.meta
	t.journal = next.journal
	return t.meta, nil
}

func (t *tailJournal) Reset(newBaseHighWaterMark uint64) (TailJournalMeta, error) {
	if t == nil || t.journal == nil {
		return TailJournalMeta{}, fmt.Errorf("reset tail journal: tail journal is nil")
	}

	journalRoot := filepath.Join(t.root, tailDirName, t.meta.ID)
	if err := t.journal.Close(); err != nil {
		return TailJournalMeta{}, fmt.Errorf("reset tail journal: close journal: %w", err)
	}

	file, err := os.OpenFile(filepath.Join(journalRoot, journalFileName), os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return TailJournalMeta{}, fmt.Errorf("reset tail journal: truncate journal file: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return TailJournalMeta{}, fmt.Errorf("reset tail journal: sync truncated journal file: %w", err)
	}
	if err := file.Close(); err != nil {
		return TailJournalMeta{}, fmt.Errorf("reset tail journal: close truncated journal file: %w", err)
	}

	journal, err := OpenJournal(journalRoot)
	if err != nil {
		return TailJournalMeta{}, fmt.Errorf("reset tail journal: reopen journal: %w", err)
	}

	t.journal = journal
	t.meta.BaseHighWaterMark = newBaseHighWaterMark
	t.meta.LastHighWaterMark = newBaseHighWaterMark
	t.meta.EventCount = 0
	t.meta.SizeBytes = 0
	return t.meta, nil
}

func newTailJournalID(baseHighWaterMark uint64) string {
	return fmt.Sprintf("tail-%020d-%d", baseHighWaterMark, time.Now().UTC().UnixNano())
}

func pruneStaleTailJournals(root, activeTailID string) error {
	if root == "" || activeTailID == "" {
		return nil
	}

	tailRoot := filepath.Join(root, tailDirName)
	entries, err := os.ReadDir(tailRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("prune stale tail journals: list tail dir: %w", err)
	}

	removed := false
	for i := range entries {
		if !entries[i].IsDir() || entries[i].Name() == activeTailID {
			continue
		}
		if err := os.RemoveAll(filepath.Join(tailRoot, entries[i].Name())); err != nil {
			return fmt.Errorf("prune stale tail journals: remove %q: %w", entries[i].Name(), err)
		}
		removed = true
	}
	if removed {
		if err := syncPath(tailRoot); err != nil {
			return fmt.Errorf("prune stale tail journals: sync tail dir: %w", err)
		}
	}

	return nil
}
