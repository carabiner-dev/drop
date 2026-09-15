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

func TestSortedRecords(t *testing.T) {
	t.Parallel()
	inv, err := inventory.OpenFile(filepath.Join(t.TempDir(), "installed.json"))
	require.NoError(t, err)
	require.Empty(t, sortedRecords(inv), "an empty inventory lists nothing")

	// Apps from two organizations, added out of order
	apps := []struct{ org, name, kind string }{
		{"otherorg", "zeta", string(ArtifactPackage)},
		{testOrg, testAppName, string(ArtifactArchive)},
		{"otherorg", "alpha", string(ArtifactBinary)},
	}
	for _, a := range apps {
		inv.Add(&inventory.Record{Host: github.DefaultHost, Org: a.org, Repo: a.name, Name: a.name, Version: "v9.9.9", Kind: a.kind})
	}
	records := sortedRecords(inv)
	require.Len(t, records, 3)
	names := []string{records[0].Name, records[1].Name, records[2].Name}
	require.Equal(t, []string{testAppName, apps[2].name, apps[0].name}, names,
		"records are ordered by repository then name")
}
