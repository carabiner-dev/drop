// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package drop

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/require"

	"github.com/carabiner-dev/drop/pkg/github"
)

// policySetAttestation returns a bare in-toto statement carrying a minimal,
// always passing policy set with the given id.
func policySetAttestation(id string) string {
	return fmt.Sprintf(`{"_type":"https://in-toto.io/Statement/v1","subject":[],`+
		`"predicateType":"https://carabiner.dev/ampel/policyset/v0.0.1",`+
		`"predicate":{"id":%q,"meta":{"description":"test","version":1},`+
		`"policies":[{"id":"pass","meta":{"description":"test","version":1},"tenets":[{"id":"t","code":"true"}]}]}}`, id)
}

// newPolicyRepo creates a committed git repository holding one policy set
// per directory, identified by the directory name.
func newPolicyRepo(t *testing.T, dirs ...string) string {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)
	wt, err := repo.Worktree()
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("policies"), 0o600))
	_, err = wt.Add("README.md")
	require.NoError(t, err)
	for _, d := range dirs {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, d), 0o750))
		file := filepath.Join(d, "policy.intoto.json")
		require.NoError(t, os.WriteFile(filepath.Join(dir, file), []byte(policySetAttestation(d)), 0o600))
		_, err = wt.Add(file)
		require.NoError(t, err)
	}
	_, err = wt.Commit("policies", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@example.com", When: time.Now()},
	})
	require.NoError(t, err)
	return dir
}

func TestFetchPolicies(t *testing.T) {
	t.Parallel()
	orgDir := OrgPolicyPath()
	repoDir := PolicyPath(testAppName)
	for _, tc := range []struct {
		name   string
		dirs   []string
		expect []string // policy set ids, in order
	}{
		{"org-and-repo", []string{orgDir, repoDir}, []string{orgDir, repoDir}},
		{"org-only", []string{orgDir}, []string{orgDir}},
		{"repo-only", []string{repoDir}, []string{repoDir}},
		{"other-repo-only", []string{PolicyPath("other")}, []string{}},
		{"no-policies", nil, []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			opts := &Options{Listener: &NoopListener{}, PolicyRepository: "file://" + newPolicyRepo(t, tc.dirs...)}
			asset := &github.Asset{Host: github.DefaultHost, Org: testOrg, Repo: testAppName}

			sets, err := (&defaultImplementation{}).FetchPolicies(opts, asset)
			require.NoError(t, err)
			ids := []string{}
			for _, set := range sets {
				ids = append(ids, set.GetId())
			}
			require.Equal(t, tc.expect, ids)
		})
	}
}

func TestFetchPoliciesMissingRepo(t *testing.T) {
	t.Parallel()
	opts := &Options{
		Listener:         &NoopListener{},
		PolicyRepository: "file://" + filepath.Join(t.TempDir(), "missing"),
	}
	asset := &github.Asset{Host: github.DefaultHost, Org: testOrg, Repo: testAppName}
	sets, err := (&defaultImplementation{}).FetchPolicies(opts, asset)
	require.NoError(t, err, "a missing policy repository means no policies")
	require.Empty(t, sets)
}

func TestFetchPoliciesCommunityFallback(t *testing.T) {
	t.Parallel()
	asset := &github.Asset{Host: github.DefaultHost, Org: testOrg, Repo: testAppName}
	communityDir := CommunityPolicyPath(testOrg, testAppName)

	for _, tc := range []struct {
		name      string
		orgDirs   []string // policy directories in the org's repo
		override  bool     // set the org repo through PolicyRepository
		community bool     // give the community repo policies for the project
		expect    []string
	}{
		{"fallback-when-org-has-none", nil, false, true, []string{communityDir}},
		{"org-policies-win", []string{OrgPolicyPath()}, false, true, []string{OrgPolicyPath()}},
		{"no-fallback-for-explicit-source", nil, true, true, []string{}},
		{"community-has-nothing-for-project", nil, false, false, []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			communityDirs := []string{CommunityPolicyPath("other", "project")}
			if tc.community {
				communityDirs = append(communityDirs, communityDir)
			}
			orgRepo := "file://" + newPolicyRepo(t, tc.orgDirs...)
			di := &defaultImplementation{
				policyRepository: func(_, _ string) string { return orgRepo },
			}
			opts := &Options{
				Listener:                  &NoopListener{},
				CommunityPolicyRepository: "file://" + newPolicyRepo(t, communityDirs...),
			}
			if tc.override {
				opts.PolicyRepository = orgRepo
			}

			sets, err := di.FetchPolicies(opts, asset)
			require.NoError(t, err)
			ids := []string{}
			for _, set := range sets {
				ids = append(ids, set.GetId())
			}
			require.Equal(t, tc.expect, ids)
		})
	}
}
