// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package github

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTokenFromEnv(t *testing.T) { //nolint:paralleltest // mutates the environment
	const (
		githubToken = "token-from-github-token" // not real credentials
		ghToken     = "token-from-gh-token"
	)
	for _, tc := range []struct {
		name       string
		github, gh string
		wantToken  string
		wantSource string
	}{
		{"none", "", "", "", ""},
		{"github-token", githubToken, "", githubToken, "GITHUB_TOKEN"},
		{"gh-token-fallback", "", ghToken, ghToken, "GH_TOKEN"},
		{"github-token-wins", githubToken, ghToken, githubToken, "GITHUB_TOKEN"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GITHUB_TOKEN", tc.github)
			t.Setenv("GH_TOKEN", tc.gh)
			token, source := tokenFromEnv()
			require.Equal(t, tc.wantToken, token)
			require.Equal(t, tc.wantSource, source)
		})
	}
}
