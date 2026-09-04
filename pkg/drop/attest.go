// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package drop

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/carabiner-dev/ampel/pkg/attest"
	"github.com/carabiner-dev/ampel/pkg/verifier"
	papi "github.com/carabiner-dev/policy/api/v1"
	util "sigs.k8s.io/release-utils/helpers"

	"github.com/carabiner-dev/drop/pkg/github"
)

// AttestationFormats lists the types of attestation drop can write about a
// verification: the full ampel result set, a SLSA verification summary
// (vsa) or an in-toto simple verification result (svr).
var AttestationFormats = verifier.ResultsAttestationFormats

// DefaultAttestationFormat is the attestation type written when none is
// specified.
const DefaultAttestationFormat = "ampel"

// ErrInvalidAttestationFormat is returned when an unknown attestation
// format is requested.
var ErrInvalidAttestationFormat = errors.New("invalid attestation format")

// Data key of attestation events
const dataKeySigned = "signed"

// attestationFileName returns the default name of the attestation of an
// app's verification: <name>-<version>.<format>.json, following the naming
// of the attestations drop publishes with its own releases.
func attestationFileName(name, version, format string) string {
	version = strings.NewReplacer("/", "_", `\`, "_").Replace(version)
	if version == "" {
		return fmt.Sprintf("%s.%s.json", name, format)
	}
	return fmt.Sprintf("%s-%s.%s.json", name, version, format)
}

// resolveAttestationPath decides where an attestation is written. An empty
// request uses the default filename in the download directory, a directory
// (existing or ending in a separator) gets the default filename inside it
// and anything else is taken as the file path.
func resolveAttestationPath(requested, dir, filename string) string {
	switch {
	case requested == "":
		return filepath.Join(dir, filename)
	case strings.HasSuffix(requested, "/"),
		strings.HasSuffix(requested, string(filepath.Separator)),
		util.IsDir(requested):
		return filepath.Join(requested, filename)
	default:
		return requested
	}
}

// AttestResults writes the attestation of the verification of an app's
// asset in the configured format, signing it when a signer is set, and
// returns the path of the file.
func (di *defaultImplementation) AttestResults(
	opts *GetOptions, name string, asset github.AssetDataProvider, results *papi.ResultSet,
) (string, error) {
	format := opts.AttestFormat
	if format == "" {
		format = DefaultAttestationFormat
	}
	if !slices.Contains(AttestationFormats, format) {
		return "", fmt.Errorf("%w: %q", ErrInvalidAttestationFormat, format)
	}

	path := resolveAttestationPath(
		opts.AttestationPath, opts.DownloadPath, attestationFileName(name, asset.GetVersion(), format),
	)

	opts.Listener.HandleEvent(&Event{
		Object: EventObjectAttestation, Verb: EventVerbRunning,
		Data: map[string]string{
			dataKeyFormat: format,
			dataKeySigned: strconv.FormatBool(opts.Signer != nil),
		},
	})

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return "", fmt.Errorf("creating attestation directory: %w", err)
	}

	attester := attest.New(attest.WithSigner(opts.Signer))
	if err := attester.AttestToFile(path, results, attest.WithFormat(format)); err != nil {
		_ = os.Remove(path) //nolint:errcheck
		return "", fmt.Errorf("writing %s attestation: %w", format, err)
	}

	opts.Listener.HandleEvent(&Event{
		Object: EventObjectAttestation, Verb: EventVerbSaved,
		Data: map[string]string{
			dataKeyPath:   path,
			dataKeyFormat: format,
		},
	})
	return path, nil
}
