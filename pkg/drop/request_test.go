// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package drop

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/carabiner-dev/drop/pkg/github"
)

const testGoreleaser = "goreleaser"

func TestPolicyRequest(t *testing.T) {
	t.Parallel()
	req := &PolicyRequest{
		Host: github.DefaultHost, Org: testGoreleaser, Repo: testGoreleaser,
		Version: "v1.2.3", Platform: "linux/amd64", Release: "v2.18.1",
	}

	require.Equal(t, "✨ Community Policies Request: goreleaser/goreleaser", req.Title())
	require.Equal(t, "https://github.com/goreleaser/goreleaser", req.RepositoryURL())
	require.Equal(t, "https://github.com/goreleaser/goreleaser/releases/tag/v2.18.1", req.ReleaseURL())

	body := req.Body()
	require.Contains(t, body, "https://github.com/goreleaser/.github", "the default policy source is named")
	require.Contains(t, body, "`ampel/policies/release/goreleaser/`", "the policy path is named")
	require.Contains(t, body, "linux/amd64")
	require.Contains(t, body, "[v2.18.1](https://github.com/goreleaser/goreleaser/releases/tag/v2.18.1)")
	require.Contains(t, body, "v1.2.3")

	u, err := url.Parse(req.URL())
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(req.URL(), DropRepositoryURL+"/issues/new?"))
	require.Equal(t, req.Title(), u.Query().Get("title"))
	require.Equal(t, body, u.Query().Get("body"))

	// An alternative policy source is reported instead of the default
	req.PolicyRepository = "https://github.com/carabiner-dev/policies"
	require.Contains(t, req.Body(), "https://github.com/carabiner-dev/policies")
	require.NotContains(t, req.Body(), ".github")

	// Without a release nothing is claimed about one
	req.Release = ""
	require.Empty(t, req.ReleaseURL())
	require.NotContains(t, req.Body(), "Release checked")
}

func TestDefaultPolicyRepository(t *testing.T) {
	t.Parallel()
	require.Equal(t, "https://github.com/goreleaser/.github", DefaultPolicyRepository("github.com", "goreleaser"))
}

func TestPolicyPath(t *testing.T) {
	t.Parallel()
	require.Equal(t, "ampel/policies/release/goreleaser", PolicyPath("goreleaser"))
}
