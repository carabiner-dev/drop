// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package inventory

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func testRecord() *Record {
	return &Record{
		Host:     "github.com",
		Org:      "carabiner-dev",
		Repo:     "drop",
		Name:     "drop",
		Version:  "v0.1.0",
		Kind:     "binary",
		Asset:    "drop-v0.1.0-linux-amd64",
		Digest:   map[string]string{"sha256": "abc123"},
		BinPath:  "/usr/local/bin/drop",
		Verified: true,
	}
}

func TestRecordKey(t *testing.T) {
	t.Parallel()
	require.Equal(t, "github.com/carabiner-dev/drop#drop", testRecord().Key())
}

func TestOpenFileMissing(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "installed.json")
	inv, err := OpenFile(path)
	require.NoError(t, err)
	require.Equal(t, Version, inv.Version)
	require.Empty(t, inv.Installs)
}

func TestRoundTrip(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "inventory", "installed.json")

	inv, err := OpenFile(path)
	require.NoError(t, err)
	record := testRecord()
	inv.Add(record)
	require.NoError(t, inv.Save())

	reloaded, err := OpenFile(path)
	require.NoError(t, err)
	require.Len(t, reloaded.Installs, 1)
	got := reloaded.Get(record.Key())
	require.NotNil(t, got)
	require.Equal(t, record.Name, got.Name)
	require.Equal(t, record.Digest, got.Digest)
	require.Equal(t, record.BinPath, got.BinPath)
	require.True(t, got.Verified)
	require.False(t, got.InstalledAt.IsZero())
	require.False(t, got.UpdatedAt.IsZero())
}

func TestAddPreservesInstalledAt(t *testing.T) {
	t.Parallel()
	inv := &Inventory{Version: Version, Installs: map[string]*Record{}}

	first := testRecord()
	first.InstalledAt = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	inv.Add(first)

	update := testRecord()
	update.Version = "v0.2.0"
	inv.Add(update)

	got := inv.Get(update.Key())
	require.Equal(t, "v0.2.0", got.Version)
	require.Equal(t, first.InstalledAt, got.InstalledAt, "installedAt must survive updates")
	require.True(t, got.UpdatedAt.After(got.InstalledAt))
}

func TestRemove(t *testing.T) {
	t.Parallel()
	inv := &Inventory{Version: Version, Installs: map[string]*Record{}}
	record := testRecord()
	inv.Add(record)

	require.True(t, inv.Remove(record.Key()))
	require.Nil(t, inv.Get(record.Key()))
	require.False(t, inv.Remove(record.Key()))
}

func TestOpenFileNewerVersion(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "installed.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"version": 99, "installs": {}}`), 0o600))

	_, err := OpenFile(path)
	require.Error(t, err)
}

func TestOpenFileCorrupt(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "installed.json")
	require.NoError(t, os.WriteFile(path, []byte("not json"), 0o600))

	_, err := OpenFile(path)
	require.Error(t, err)
}
