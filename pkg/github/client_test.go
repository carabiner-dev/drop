// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package github

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseAssetString(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		input  string
		expect *Asset
	}{
		{
			"full", "github.com/carabiner-dev/drop@v1.0.0#installer",
			&Asset{Host: "github.com", Org: "carabiner-dev", Repo: "drop", Version: "v1.0.0", Name: "installer"},
		},
		{
			"slug", "carabiner-dev/drop",
			&Asset{Org: "carabiner-dev", Repo: "drop"},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := NewAssetFromString(tc.input)
			require.Equal(t, tc.expect, a)
		})
	}
}
