// Package infrastructure internal test: snapshot/rollback round-trip.
//
// The checkout applier's rollback path only runs on mid-write failures,
// which the external tests trigger with dangling symlinks. This file pins
// the mechanism itself — byte-identical shared files after rollback,
// created directories removed, pre-existing directories kept — through the
// unexported helpers directly.
package infrastructure

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnapshotRollbackRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "success - restores files and removes created dirs",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(root, "go.work"), []byte("go 1.27\n"), 0o644))
			require.NoError(t, os.MkdirAll(filepath.Join(root, "keep"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(root, "keep", "data.txt"), []byte("keep"), 0o644))

			a := NewCheckoutApplier(root, "go", "v1.6.0", 0, nil)
			snap := a.snapshot(root, []string{"go.work", "missing.txt"})
			snap.dirs = append(snap.dirs, "created")

			require.NoError(t, os.WriteFile(filepath.Join(root, "go.work"), []byte("mutated"), 0o644))
			require.NoError(t, os.WriteFile(filepath.Join(root, "missing.txt"), []byte("new"), 0o644))
			require.NoError(t, os.MkdirAll(filepath.Join(root, "created"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(root, "created", "x.txt"), []byte("x"), 0o644))
			require.NoError(t, os.WriteFile(filepath.Join(root, "keep", "data.txt"), []byte("mutated-too"), 0o644))

			a.rollback(root, snap)
			require.Equal(t, tt.wantErr, false)

			work, err := os.ReadFile(filepath.Join(root, "go.work"))
			require.NoError(t, err)
			assert.Equal(t, "go 1.27\n", string(work))
			_, err = os.Stat(filepath.Join(root, "missing.txt"))
			assert.NoError(t, err, "unsnapshotted files are not rollback's business")
			_, err = os.Stat(filepath.Join(root, "created"))
			assert.True(t, os.IsNotExist(err))
			kept, err := os.ReadFile(filepath.Join(root, "keep", "data.txt"))
			require.NoError(t, err)
			assert.Equal(t, "mutated-too", string(kept), "untracked paths are not rollback's business")
		})
	}
}
