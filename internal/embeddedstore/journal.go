package embeddedstore

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/moolen/spectre/internal/models"
)

const (
	journalFileName      = "events.journal"
	maxJournalRecordSize = 8 * 1024 * 1024 // 8 MiB
)

var errJournalClosed = errors.New("journal is closed")

var (
	syncPathFnMu sync.RWMutex
	syncPathFn   = syncPathDirect
)

// Journal stores events in an append-only on-disk log.
// Callers must ensure a single writer per journal path across processes; no file lock is taken.
type Journal struct {
	mu   sync.Mutex
	path string
	file *os.File
}

// OpenJournal opens (or creates) a journal rooted at the provided directory.
func OpenJournal(root string) (*Journal, error) {
	if root == "" {
		return nil, fmt.Errorf("open journal: root path is empty")
	}

	rootCreated, err := pathCreated(root)
	if err != nil {
		return nil, fmt.Errorf("open journal: stat root dir: %w", err)
	}

	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("open journal: create root dir: %w", err)
	}
	if rootCreated {
		if err := syncPath(filepath.Dir(root)); err != nil {
			return nil, fmt.Errorf("open journal: sync parent dir: %w", err)
		}
	}

	path := filepath.Join(root, journalFileName)
	fileCreated, err := pathCreated(path)
	if err != nil {
		return nil, fmt.Errorf("open journal: stat file: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open journal: open file: %w", err)
	}
	if fileCreated {
		if err := syncPath(root); err != nil {
			_ = file.Close()
			return nil, fmt.Errorf("open journal: sync root dir: %w", err)
		}
	}

	return &Journal{
		path: path,
		file: file,
	}, nil
}

// Close releases the underlying file handle.
func (j *Journal) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.file == nil {
		return nil
	}

	if err := j.file.Close(); err != nil {
		return fmt.Errorf("close journal: %w", err)
	}
	j.file = nil

	return nil
}

// Append appends one event durably.
func (j *Journal) Append(ctx context.Context, event models.Event) error {
	return j.AppendBatch(ctx, []models.Event{event})
}

// AppendBatch appends events durably in order.
func (j *Journal) AppendBatch(ctx context.Context, events []models.Event) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(events) == 0 {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	j.mu.Lock()
	defer j.mu.Unlock()
	if j.file == nil {
		return errJournalClosed
	}

	var writeBuf bytes.Buffer
	for i := range events {
		if err := ctx.Err(); err != nil {
			return err
		}

		payload, err := json.Marshal(events[i])
		if err != nil {
			return fmt.Errorf("append journal entry: marshal event: %w", err)
		}
		if len(payload) > maxJournalRecordSize {
			return fmt.Errorf("append journal entry: payload size %d exceeds max %d", len(payload), maxJournalRecordSize)
		}

		var header [4]byte
		binary.BigEndian.PutUint32(header[:], uint32(len(payload)))
		if _, err := writeBuf.Write(header[:]); err != nil {
			return fmt.Errorf("append journal entry: encode length: %w", err)
		}
		if _, err := writeBuf.Write(payload); err != nil {
			return fmt.Errorf("append journal entry: encode payload: %w", err)
		}
	}

	if _, err := j.file.Write(writeBuf.Bytes()); err != nil {
		return fmt.Errorf("append journal batch: write: %w", err)
	}
	if err := j.file.Sync(); err != nil {
		return fmt.Errorf("append journal batch: sync: %w", err)
	}

	return nil
}

// Replay replays all events in append order.
func (j *Journal) Replay(ctx context.Context) ([]models.Event, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	j.mu.Lock()
	defer j.mu.Unlock()
	if j.file == nil {
		return nil, errJournalClosed
	}

	readFile, err := os.Open(j.path)
	if err != nil {
		return nil, fmt.Errorf("replay journal: open file: %w", err)
	}
	defer func() {
		_ = readFile.Close()
	}()

	events := make([]models.Event, 0)
	recordIndex := 0

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		var header [4]byte
		_, err := io.ReadFull(readFile, header[:])
		if err == nil {
			// continue parsing below
		} else if errors.Is(err, io.EOF) {
			return events, nil
		} else if errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, fmt.Errorf("truncated journal entry header at record %d: %w", recordIndex, err)
		} else {
			return nil, fmt.Errorf("replay journal: read entry header at record %d: %w", recordIndex, err)
		}

		length := binary.BigEndian.Uint32(header[:])
		if length > uint32(maxJournalRecordSize) {
			return nil, fmt.Errorf("oversized journal entry payload at record %d: size %d exceeds max %d", recordIndex, length, maxJournalRecordSize)
		}
		payload := make([]byte, int(length))
		if _, err := io.ReadFull(readFile, payload); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return nil, fmt.Errorf("truncated journal entry payload at record %d: %w", recordIndex, err)
			}
			return nil, fmt.Errorf("replay journal: read entry payload at record %d: %w", recordIndex, err)
		}

		var event models.Event
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, fmt.Errorf("corrupt journal entry at record %d: %w", recordIndex, err)
		}

		events = append(events, event)
		recordIndex++
	}
}

func pathCreated(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return false, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	return false, err
}

func syncPath(path string) error {
	syncPathFnMu.RLock()
	fn := syncPathFn
	syncPathFnMu.RUnlock()

	return fn(path)
}

func syncPathDirect(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = dir.Close()
	}()

	return dir.Sync()
}

func setSyncPathFnForTest(fn func(string) error) func() {
	syncPathFnMu.Lock()
	previous := syncPathFn
	syncPathFn = fn
	syncPathFnMu.Unlock()

	return func() {
		syncPathFnMu.Lock()
		syncPathFn = previous
		syncPathFnMu.Unlock()
	}
}
