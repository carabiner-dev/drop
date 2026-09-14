// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package github

import (
	"testing"

	gogithub "github.com/google/go-github/v60/github"
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
			&Asset{Host: DefaultHost, Org: "carabiner-dev", Repo: "drop", Version: "v1.0.0", Name: "installer"},
		},
		{
			"slug", "carabiner-dev/drop",
			&Asset{Org: "carabiner-dev", Repo: "drop"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := NewAssetFromURLString(tc.input)
			require.Equal(t, tc.expect, a)
		})
	}
}

func TestRepoURLFromString(t *testing.T) {
	t.Parallel()
	const cosignRepoURL = "https://github.com/sigstore/cosign"
	for _, tc := range []struct {
		name    string
		sut     string
		expect  string
		mustErr bool
	}{
		{"reposlug", "sigstore/cosign", cosignRepoURL, false},
		{"noscheme", "github.com/sigstore/cosign", cosignRepoURL, false},
		{"norepo", "github.com/sigstore", "", true},
		{"locator", "git+https://github.com/sigstore/cosign@main", cosignRepoURL, false},
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

func TestLatestRelease(t *testing.T) {
	t.Parallel()
	rel := func(tag string, prerelease, draft bool) *gogithub.RepositoryRelease {
		return &gogithub.RepositoryRelease{
			TagName: gogithub.String(tag), Prerelease: gogithub.Bool(prerelease), Draft: gogithub.Bool(draft),
		}
	}
	for _, tc := range []struct {
		name     string
		releases []*gogithub.RepositoryRelease
		expected string // "" = nil
	}{
		{"empty", nil, ""},
		{"stable-first", []*gogithub.RepositoryRelease{rel("v2.0.0", false, false), rel("v1.0.0", false, false)}, "v2.0.0"},
		{
			"skip-nightlies",
			[]*gogithub.RepositoryRelease{
				rel("v2.19.0-2ab31000-nightly", true, false),
				rel("v2.19.0-a95c9a0d-nightly", true, false),
				rel("v2.18.1", false, false),
				rel("v2.18.0", false, false),
			},
			"v2.18.1",
		},
		{"skip-drafts", []*gogithub.RepositoryRelease{rel("v3.0.0", false, true), rel("v2.0.0", false, false)}, "v2.0.0"},
		{"only-prereleases", []*gogithub.RepositoryRelease{rel("v1.0.0-rc2", true, false), rel("v1.0.0-rc1", true, false)}, "v1.0.0-rc2"},
		{"only-drafts", []*gogithub.RepositoryRelease{rel("v1.0.0", false, true)}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res := latestRelease(tc.releases)
			if tc.expected == "" {
				require.Nil(t, res)
				return
			}
			require.NotNil(t, res)
			require.Equal(t, tc.expected, res.GetTagName())
		})
	}
}
