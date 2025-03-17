// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package github

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewAssetFromURLString(t *testing.T) {
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
			a := NewAssetFromURLString(tc.input)
			require.Equal(t, tc.expect, a)
		})
	}
}

func TestRepoURLFromString(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		sut     string
		expect  string
		mustErr bool
	}{
		{"reposlug", "sigstore/cosign", "https://github.com/sigstore/cosign", false},
		{"noscheme", "github.com/sigstore/cosign", "https://github.com/sigstore/cosign", false},
		{"norepo", "github.com/sigstore", "", true},
		{"locator", "git+https://github.com/sigstore/cosign@main", "https://github.com/sigstore/cosign", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res, err := RepoURLFromString(tc.sut)
			if tc.mustErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expect, res)
		})
	}
}
