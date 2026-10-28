// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package drop

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/carabiner-dev/drop/pkg/github"
	"github.com/carabiner-dev/drop/pkg/inventory"
)

const (
	cosignName  = "cosign"
	cosignKey   = "github.com/sigstore/cosign#cosign"
	cosignSBOM  = "github.com/sigstore/cosign#cosign-sbom"
	forkedKey   = "git.example.com/forks/cosign#cosign"
	exampleHost = "git.example.com"
	sigstoreOrg = "sigstore"
)

// removeTestInventory writes an inventory with apps from three
// repositories, two of them providing an app of the same name.
func removeTestInventory(t *testing.T) (inv *inventory.Inventory, path string) {
	t.Helper()
	path = filepath.Join(t.TempDir(), "installed.json")
	inv, err := inventory.OpenFile(path)
	require.NoError(t, err)
	for _, a := range []struct{ host, org, repo, name string }{
		{github.DefaultHost, testOrg, testAppName, testAppName},
		{github.DefaultHost, sigstoreOrg, cosignName, cosignName},
		{github.DefaultHost, sigstoreOrg, cosignName, "cosign-sbom"},
		{exampleHost, "forks", cosignName, cosignName},
	} {
		inv.Add(&inventory.Record{Host: a.host, Org: a.org, Repo: a.repo, Name: a.name, Version: v100, Kind: string(ArtifactBinary)})
	}
	return inv, path
}

func TestMatchInstalled(t *testing.T) {
	t.Parallel()
	inv, _ := removeTestInventory(t)
	for _, tc := range []struct {
		name   string
		ref    string
		expect []string // keys
	}{
		{name: "by-name", ref: testAppName, expect: []string{"github.com/carabiner-dev/drop#drop"}},
		{name: "name-in-two-repos", ref: cosignName, expect: []string{forkedKey, cosignKey}},
		{name: "by-repo", ref: "sigstore/cosign", expect: []string{cosignKey, cosignSBOM}},
		{name: "by-host-repo", ref: exampleHost + "/forks/cosign", expect: []string{forkedKey}},
		{name: "url", ref: "https://github.com/sigstore/cosign/", expect: []string{cosignKey, cosignSBOM}},
		{name: "unknown", ref: "nothing"},
		{name: "blank", ref: "  "},
		{name: "wrong-host", ref: "gitlab.com/sigstore/cosign"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			keys := []string{}
			for _, r := range matchInstalled(inv, tc.ref) {
				keys = append(keys, r.Key())
			}
			if tc.expect == nil {
				require.Empty(t, keys)
				return
			}
			require.Equal(t, tc.expect, keys)
		})
	}
}

func TestForgetInstall(t *testing.T) {
	t.Parallel()
	inv, path := removeTestInventory(t)
	require.NoError(t, inv.Save())
	di := &defaultImplementation{inventoryPath: path}
	opts := &GetOptions{}

	record := inv.Get(cosignKey)
	require.NotNil(t, record)
	require.NoError(t, di.ForgetInstall(opts, record))

	reloaded, err := inventory.OpenFile(path)
	require.NoError(t, err)
	require.Nil(t, reloaded.Get(record.Key()), "the record is gone from disk")
	require.Len(t, reloaded.Installs, 3, "the other apps stay")

	// Forgetting twice is not an error
	require.NoError(t, di.ForgetInstall(opts, record))
}

func TestRemovingPrevious(t *testing.T) {
	t.Parallel()
	record := &inventory.Record{Host: github.DefaultHost, Org: testOrg, Repo: testAppName, Name: testAppName}
	other := &inventory.Record{Host: github.DefaultHost, Org: sigstoreOrg, Repo: cosignName, Name: cosignName}
	require.False(t, removingPrevious(&GetOptions{}, record), "no reinstall in progress")
	require.True(t, removingPrevious(&GetOptions{previous: record}, record))
	require.False(t, removingPrevious(&GetOptions{previous: other}, record))
}
