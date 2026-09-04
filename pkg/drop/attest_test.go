// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package drop

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	papi "github.com/carabiner-dev/policy/api/v1"
	"github.com/carabiner-dev/signer"
	"github.com/carabiner-dev/signer/key"
	"github.com/carabiner-dev/signer/options"
	intoto "github.com/in-toto/attestation/go/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/carabiner-dev/drop/pkg/github"
)

const (
	testAssetURL = "https://github.com/carabiner-dev/drop/releases/download/v0.1.0/drop-linux-amd64"
	testVersion  = "v0.1.0"
	formatVSA    = "vsa"
	formatSVR    = "svr"
	algoSHA256   = "sha256"

	predicateTypeResultSet = "https://carabiner.dev/ampel/resultset/v0"
	predicateTypeVSA       = "https://slsa.dev/verification_summary/v1"
	predicateTypeSVR       = "https://in-toto.io/attestation/svr/v0.1"
)

func TestAttestationFileName(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, version, format, expect string
	}{
		{"gh", "v2.1.0", formatVSA, "gh-v2.1.0.vsa.json"},
		{"drop", testVersion, DefaultAttestationFormat, "drop-v0.1.0.ampel.json"},
		{"kit", "release/2024.1", formatSVR, "kit-release_2024.1.svr.json"},
		{"box", `weird\tag`, formatSVR, "box-weird_tag.svr.json"},
		{"cup", "", DefaultAttestationFormat, "cup.ampel.json"},
	} {
		t.Run(tc.expect, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expect, attestationFileName(tc.name, tc.version, tc.format))
		})
	}
}

func TestResolveAttestationPath(t *testing.T) {
	t.Parallel()
	const filename = "drop-v0.1.0.ampel.json"
	existingDir := t.TempDir()
	missing := filepath.Join(t.TempDir(), "missing")

	for _, tc := range []struct {
		name      string
		requested string
		expect    string
	}{
		{"default-in-download-dir", "", filepath.Join("downloads", filename)},
		{"existing-dir", existingDir, filepath.Join(existingDir, filename)},
		{"trailing-slash-missing-dir", missing + "/", filepath.Join(missing, filename)},
		{"trailing-separator", missing + string(filepath.Separator), filepath.Join(missing, filename)},
		{"file-path", filepath.Join(missing, "custom.json"), filepath.Join(missing, "custom.json")},
		{"bare-file-name", "custom.json", "custom.json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expect, resolveAttestationPath(tc.requested, "downloads", filename))
		})
	}
}

// testResultSet builds a finished result set for a verified asset
func testResultSet(status string) *papi.ResultSet {
	subject := &intoto.ResourceDescriptor{
		Name:   testBinFile,
		Uri:    testAssetURL,
		Digest: map[string]string{algoSHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
	}
	now := timestamppb.Now()
	return &papi.ResultSet{
		PolicySet: &papi.PolicyRef{Id: "release-policy", Version: 1},
		Meta:      &papi.PolicySetMeta{Version: 1},
		Status:    status,
		DateStart: now,
		DateEnd:   now,
		Subject:   subject,
		Results: []*papi.Result{{
			Policy:    &papi.PolicyRef{Id: "signed-release", Version: 1},
			Status:    status,
			DateStart: now,
			DateEnd:   now,
			Subject:   subject,
		}},
	}
}

func testAsset() *github.Asset {
	return &github.Asset{
		Host: "github.com", Org: "carabiner-dev", Repo: testAppName,
		Version: testVersion, Name: testBinFile, DownloadURL: testAssetURL,
	}
}

// newKeySigner returns a signer using a fresh key so signing needs no
// network access. Key signing yields a DSSE envelope.
func newKeySigner(t *testing.T) *signer.Signer {
	t.Helper()
	priv, err := key.NewGenerator().GenerateKeyPair()
	require.NoError(t, err)
	s := signer.NewSigner()
	s.Options.Backend = options.BackendKey
	s.Options.Keys = []key.PrivateKeyProvider{priv}
	return s
}

// statement is the part of an in-toto statement the tests look at
type statement struct {
	Type          string `json:"_type"`
	PredicateType string `json:"predicateType"`
	Subject       []struct {
		Name   string            `json:"name"`
		URI    string            `json:"uri"`
		Digest map[string]string `json:"digest"`
	} `json:"subject"`
	Predicate map[string]any `json:"predicate"`
}

func readStatement(t *testing.T, data []byte) *statement {
	t.Helper()
	stmt := &statement{}
	require.NoError(t, json.Unmarshal(data, stmt))
	require.Equal(t, "https://in-toto.io/Statement/v1", stmt.Type)
	require.Len(t, stmt.Subject, 1)
	require.Equal(t, testBinFile, stmt.Subject[0].Name)
	require.Equal(t, testAssetURL, stmt.Subject[0].URI, "the subject points at the release asset")
	require.NotEmpty(t, stmt.Subject[0].Digest[algoSHA256])
	return stmt
}

func TestAttestResultsFormats(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		format        string
		status        string
		predicateType string
		check         func(t *testing.T, predicate map[string]any)
	}{
		{
			format: DefaultAttestationFormat, status: papi.StatusPASS, predicateType: predicateTypeResultSet,
			check: func(t *testing.T, predicate map[string]any) {
				t.Helper()
				require.Equal(t, papi.StatusPASS, predicate["status"])
			},
		},
		{
			format: formatVSA, status: papi.StatusPASS, predicateType: predicateTypeVSA,
			check: func(t *testing.T, predicate map[string]any) {
				t.Helper()
				require.Equal(t, "PASSED", predicate["verificationResult"])
				require.Equal(t, testAssetURL, predicate["resourceUri"])
			},
		},
		{
			format: formatVSA, status: papi.StatusFAIL, predicateType: predicateTypeVSA,
			check: func(t *testing.T, predicate map[string]any) {
				t.Helper()
				require.Equal(t, "FAILED", predicate["verificationResult"], "a failed verification is attested as such")
			},
		},
		{
			format: formatSVR, status: papi.StatusPASS, predicateType: predicateTypeSVR,
			check: func(t *testing.T, predicate map[string]any) {
				t.Helper()
				require.Contains(t, predicate, "verifier")
			},
		},
	} {
		t.Run(tc.format+"-"+tc.status, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			events := &recorder{}
			opts := &GetOptions{DownloadPath: dir}
			opts.Listener = events
			opts.AttestFormat = tc.format

			di := &defaultImplementation{}
			path, err := di.AttestResults(opts, testAppName, testAsset(), testResultSet(tc.status))
			require.NoError(t, err)
			require.Equal(t, filepath.Join(dir, "drop-v0.1.0."+tc.format+".json"), path)

			data, err := os.ReadFile(path) //nolint:gosec // test path
			require.NoError(t, err)
			stmt := readStatement(t, data)
			require.Equal(t, tc.predicateType, stmt.PredicateType)
			tc.check(t, stmt.Predicate)

			running := events.find(EventObjectAttestation, EventVerbRunning)
			require.NotNil(t, running)
			require.Equal(t, tc.format, running.GetDataField(dataKeyFormat))
			require.Equal(t, "false", running.GetDataField(dataKeySigned))
			saved := events.find(EventObjectAttestation, EventVerbSaved)
			require.NotNil(t, saved)
			require.Equal(t, path, saved.GetDataField(dataKeyPath))
		})
	}
}

func TestAttestResultsSigned(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	events := &recorder{}
	opts := &GetOptions{DownloadPath: dir}
	opts.Listener = events
	opts.AttestFormat = formatVSA
	opts.Signer = newKeySigner(t)

	di := &defaultImplementation{}
	path, err := di.AttestResults(opts, testAppName, testAsset(), testResultSet(papi.StatusPASS))
	require.NoError(t, err)

	data, err := os.ReadFile(path) //nolint:gosec // test path
	require.NoError(t, err)

	// Key signing produces a DSSE envelope wrapping the statement
	envelope := struct {
		PayloadType string `json:"payloadType"`
		Payload     string `json:"payload"`
		Signatures  []struct {
			Sig string `json:"sig"`
		} `json:"signatures"`
	}{}
	require.NoError(t, json.Unmarshal(data, &envelope))
	require.Equal(t, "https://in-toto.io/Statement/v1", envelope.PayloadType)
	require.NotEmpty(t, envelope.Signatures)
	require.NotEmpty(t, envelope.Signatures[0].Sig)

	payload, err := base64.StdEncoding.DecodeString(envelope.Payload)
	require.NoError(t, err)
	stmt := readStatement(t, payload)
	require.Equal(t, predicateTypeVSA, stmt.PredicateType)
	require.Equal(t, "true", events.find(EventObjectAttestation, EventVerbRunning).GetDataField(dataKeySigned))
}

func TestAttestResultsPaths(t *testing.T) {
	t.Parallel()
	t.Run("directory", func(t *testing.T) {
		t.Parallel()
		out := t.TempDir()
		opts := &GetOptions{DownloadPath: t.TempDir(), AttestationPath: out}
		opts.Listener = &NoopListener{}
		di := &defaultImplementation{}
		path, err := di.AttestResults(opts, testAppName, testAsset(), testResultSet(papi.StatusPASS))
		require.NoError(t, err)
		require.Equal(t, filepath.Join(out, "drop-v0.1.0.ampel.json"), path)
		require.FileExists(t, path)
	})

	t.Run("file-in-missing-directory", func(t *testing.T) {
		t.Parallel()
		out := filepath.Join(t.TempDir(), "evidence", "drop.json")
		opts := &GetOptions{DownloadPath: t.TempDir(), AttestationPath: out}
		opts.Listener = &NoopListener{}
		di := &defaultImplementation{}
		path, err := di.AttestResults(opts, testAppName, testAsset(), testResultSet(papi.StatusPASS))
		require.NoError(t, err)
		require.Equal(t, out, path)
		require.FileExists(t, path)
	})

	t.Run("overwrites", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		opts := &GetOptions{DownloadPath: dir}
		opts.Listener = &NoopListener{}
		di := &defaultImplementation{}
		for range 2 {
			_, err := di.AttestResults(opts, testAppName, testAsset(), testResultSet(papi.StatusPASS))
			require.NoError(t, err)
		}
		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Len(t, entries, 1)
	})

	t.Run("invalid-format", func(t *testing.T) {
		t.Parallel()
		opts := &GetOptions{DownloadPath: t.TempDir()}
		opts.Listener = &NoopListener{}
		opts.AttestFormat = "yaml"
		di := &defaultImplementation{}
		_, err := di.AttestResults(opts, testAppName, testAsset(), testResultSet(papi.StatusPASS))
		require.ErrorIs(t, err, ErrInvalidAttestationFormat)
	})
}

func TestWithAttestation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		format    string
		expect    string
		expectErr bool
	}{
		{"", DefaultAttestationFormat, false},
		{DefaultAttestationFormat, DefaultAttestationFormat, false},
		{formatVSA, formatVSA, false},
		{formatSVR, formatSVR, false},
		{"yaml", "", true},
	} {
		t.Run("format-"+tc.format, func(t *testing.T) {
			t.Parallel()
			d := &Dropper{}
			err := WithAttestation(tc.format)(d)
			if tc.expectErr {
				require.ErrorIs(t, err, ErrInvalidAttestationFormat)
				require.False(t, d.Options.Attest)
				return
			}
			require.NoError(t, err)
			require.True(t, d.Options.Attest)
			require.Equal(t, tc.expect, d.Options.AttestFormat)
		})
	}
}
