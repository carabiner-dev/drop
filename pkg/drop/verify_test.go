// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package drop

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	papi "github.com/carabiner-dev/policy/api/v1"
	intoto "github.com/in-toto/attestation/go/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/carabiner-dev/drop/pkg/github"
)

func TestAssetSubject(t *testing.T) {
	t.Parallel()
	content := []byte("release asset data")
	sum := sha256.Sum256(content)
	wantDigest := hex.EncodeToString(sum[:])

	filePath := filepath.Join(t.TempDir(), "drop")
	require.NoError(t, os.WriteFile(filePath, content, 0o600))

	t.Run("release-asset", func(t *testing.T) {
		t.Parallel()
		asset := &github.Asset{
			Name:        testBinFile,
			DownloadURL: "https://github.com/carabiner-dev/drop/releases/download/v0.1.0/" + testBinFile,
		}
		subject, err := assetSubject(asset, filePath)
		require.NoError(t, err)
		require.Equal(t, testBinFile, subject.GetName(), "the subject is named after the asset")
		require.Equal(t, asset.DownloadURL, subject.GetUri(), "the subject points at the release, not the local copy")
		require.Equal(t, wantDigest, subject.GetDigest()["sha256"])
		require.Contains(t, subject.GetDigest(), "sha512")
	})

	t.Run("bare-spec-keeps-local-identity", func(t *testing.T) {
		t.Parallel()
		subject, err := assetSubject(&github.Asset{}, filePath)
		require.NoError(t, err)
		require.Equal(t, "drop", subject.GetName())
		require.Equal(t, filePath, subject.GetUri())
		require.Equal(t, wantDigest, subject.GetDigest()["sha256"])
	})

	t.Run("missing-file", func(t *testing.T) {
		t.Parallel()
		_, err := assetSubject(&github.Asset{Name: testBinFile}, filepath.Join(t.TempDir(), "nope"))
		require.Error(t, err)
	})
}

func TestFinalizeResultSet(t *testing.T) {
	t.Parallel()
	subject := &intoto.ResourceDescriptor{
		Name:   testBinFile,
		Uri:    "https://example.com/" + testBinFile,
		Digest: map[string]string{"sha256": "abc"},
	}
	policySet := func(id string, version int64) *papi.PolicySet {
		return &papi.PolicySet{Id: id, Meta: &papi.PolicySetMeta{Version: version}}
	}

	for _, tc := range []struct {
		name         string
		rs           *papi.ResultSet
		policies     []*papi.PolicySet
		passed       bool
		expectStatus string
		expectRef    *papi.PolicyRef
	}{
		{
			name: "passed", rs: &papi.ResultSet{}, passed: true, expectStatus: papi.StatusPASS,
		},
		{
			name: "failed", rs: &papi.ResultSet{}, passed: false, expectStatus: papi.StatusFAIL,
		},
		{
			name:   "drop-verdict-wins",
			rs:     &papi.ResultSet{Status: papi.StatusPASS},
			passed: false, expectStatus: papi.StatusFAIL,
		},
		{
			name:     "single-policy-set-is-referenced",
			rs:       &papi.ResultSet{},
			policies: []*papi.PolicySet{policySet("release-policy", 3)},
			passed:   true, expectStatus: papi.StatusPASS,
			expectRef: &papi.PolicyRef{Id: "release-policy", Version: 3},
		},
		{
			name:     "several-policy-sets-are-not",
			rs:       &papi.ResultSet{},
			policies: []*papi.PolicySet{policySet("a", 1), policySet("b", 1)},
			passed:   true, expectStatus: papi.StatusPASS,
		},
		{
			name:     "existing-reference-kept",
			rs:       &papi.ResultSet{PolicySet: &papi.PolicyRef{Id: "already", Version: 9}},
			policies: []*papi.PolicySet{policySet("other", 1)},
			passed:   true, expectStatus: papi.StatusPASS,
			expectRef: &papi.PolicyRef{Id: "already", Version: 9},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			start := timestamppb.New(time.Now().Add(-time.Minute))
			finalizeResultSet(tc.rs, subject, tc.policies, start, tc.passed)

			require.Equal(t, tc.expectStatus, tc.rs.GetStatus())
			require.Same(t, subject, tc.rs.GetSubject())
			require.Same(t, start, tc.rs.GetDateStart())
			require.NotNil(t, tc.rs.GetDateEnd())
			require.False(t, tc.rs.GetDateEnd().AsTime().Before(start.AsTime()))

			if tc.expectRef == nil {
				require.Nil(t, tc.rs.GetPolicySet())
				require.Nil(t, tc.rs.GetMeta())
				return
			}
			require.Equal(t, tc.expectRef.GetId(), tc.rs.GetPolicySet().GetId())
			require.Equal(t, tc.expectRef.GetVersion(), tc.rs.GetPolicySet().GetVersion())
		})
	}

	t.Run("dates-are-kept-when-set", func(t *testing.T) {
		t.Parallel()
		startSet := timestamppb.New(time.Now().Add(-time.Hour))
		endSet := timestamppb.New(time.Now().Add(-time.Minute))
		rs := &papi.ResultSet{DateStart: startSet, DateEnd: endSet}
		finalizeResultSet(rs, subject, nil, timestamppb.Now(), true)
		require.Same(t, startSet, rs.GetDateStart())
		require.Same(t, endSet, rs.GetDateEnd())
	})
}
