package embeddedstore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestManifestStore_LoadOrCreateManifestCreatesInitialManifest(t *testing.T) {
	dir := t.TempDir()

	manifest, err := loadOrCreateManifest(dir)
	require.NoError(t, err)
	require.Equal(t, storageFormatVersion, manifest.FormatVersion)
	require.Empty(t, manifest.ActiveSegments)
	require.Empty(t, manifest.Checkpoints)
}

func TestManifestStore_StoreManifestAtomicallyReplacesManifest(t *testing.T) {
	dir := t.TempDir()

	_, err := loadOrCreateManifest(dir)
	require.NoError(t, err)

	err = storeManifest(dir, Manifest{
		FormatVersion: storageFormatVersion,
		ActiveSegments: []SegmentMeta{
			{ID: "seg-001"},
		},
		Checkpoints: []CheckpointMeta{},
	})
	require.NoError(t, err)

	reloaded, err := loadOrCreateManifest(dir)
	require.NoError(t, err)
	require.Len(t, reloaded.ActiveSegments, 1)
	require.Equal(t, "seg-001", reloaded.ActiveSegments[0].ID)
}

func TestManifestStore_LoadOrCreateManifestRejectsVersionMismatch(t *testing.T) {
	dir := t.TempDir()

	manifestPath := filepath.Join(dir, manifestFileName)
	err := os.WriteFile(manifestPath, []byte(`{"format_version":2,"active_segments":[],"checkpoints":[]}`), 0o600)
	require.NoError(t, err)

	_, err = loadOrCreateManifest(dir)
	require.Error(t, err)
	require.ErrorContains(t, err, "unsupported manifest format version")
}
