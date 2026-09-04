// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/carabiner-dev/signer"
	signerOpts "github.com/carabiner-dev/signer/options"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/carabiner-dev/drop/pkg/drop"
)

// attestOptions holds the flags controlling the attestation of the
// verifications drop performs, including the carabiner signer options.
type attestOptions struct {
	// Attest enables writing an attestation of the verification
	Attest bool

	// Type is the attestation type to write (ampel, vsa or svr)
	Type string

	// Out is the file or directory the attestation is written to
	Out string

	// Sign signs the attestation, unsigned statements are written otherwise
	Sign bool

	// SignerSet holds the signing flags (backend, keys, sigstore, spiffe)
	*signerOpts.SignerSet
}

// defaultAttestOptions returns the attestation options with their defaults:
// attesting off, the ampel format, signed with the default signer set
// (sigstore, trying ambient credentials before the interactive flow).
func defaultAttestOptions() *attestOptions {
	return &attestOptions{
		Type:      drop.DefaultAttestationFormat,
		Sign:      true,
		SignerSet: signerOpts.DefaultSignerSet(),
	}
}

// Flag groups of the attestation options in the usage text
const (
	grpAttestation = "attestation"
	grpSigning     = "signing"
)

// AddFlags adds the attestation and signing flags to a command, rendered
// under their own headings in the usage text.
func (ao *attestOptions) AddFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().BoolVar(
		&ao.Attest, "attest", false, "write an attestation of the verification performed",
	)

	cmd.PersistentFlags().StringVar(
		&ao.Type, "attestation-type", drop.DefaultAttestationFormat,
		fmt.Sprintf("type of attestation to write %v", drop.AttestationFormats),
	)

	cmd.PersistentFlags().StringVar(
		&ao.Out, "attestation-out", "",
		"file or directory to write the attestation to (default: <app>-<version>.<type>.json next to the download)",
	)

	cmd.PersistentFlags().BoolVar(
		&ao.Sign, "sign", true,
		"sign the attestation (sigstore opens a browser or uses ambient CI credentials, and records the signature in the public Rekor log)",
	)

	ao.SignerSet.AddFlags(cmd)

	groupFlags(cmd, grpAttestation, "attest", "attestation-type", "attestation-out", "sign")
	groupFlagsByPrefix(cmd, grpSigning, "signing-", "sigstore-", "spiffe-")
	registerFlagGroups(cmd,
		flagGroup{ID: grpAttestation, Title: "Attestation Flags:"},
		flagGroup{ID: grpSigning, Title: "Signing Flags:"},
	)
}

// Validate checks the attestation flags. insecure reports if verification
// is skipped, in which case there is nothing to attest.
func (ao *attestOptions) Validate(insecure bool) error {
	errs := []error{}
	if !ao.Attest {
		if ao.Out != "" {
			errs = append(errs, errors.New("--attestation-out requires --attest"))
		}
		if !ao.Sign {
			errs = append(errs, errors.New("--sign=false requires --attest"))
		}
		return errors.Join(errs...)
	}

	if insecure {
		errs = append(errs, errors.New("--attest cannot be combined with --insecure, there is no verification to attest"))
	}

	if !slices.Contains(drop.AttestationFormats, ao.Type) {
		errs = append(errs, fmt.Errorf("invalid attestation type %q, valid types are %v", ao.Type, drop.AttestationFormats))
	}

	if ao.Sign {
		if err := ao.SignerSet.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("signing options: %w", err))
		}
	}
	return errors.Join(errs...)
}

// DropperOptions returns the dropper options enabling the attestation and a
// function releasing the signer once done. When signing with sigstore, the
// credentials are obtained up front so the user authenticates (or the run
// fails for lack of ambient credentials) before anything is downloaded.
func (ao *attestOptions) DropperOptions(ctx context.Context) ([]drop.FuncOption, func(), error) {
	noop := func() {}
	if !ao.Attest {
		return nil, noop, nil
	}

	opts := make([]drop.FuncOption, 0, 2)
	opts = append(opts, drop.WithAttestation(ao.Type))
	if !ao.Sign {
		return opts, noop, nil
	}

	s, err := signer.NewSignerFromSet(ao.SignerSet)
	if err != nil {
		return nil, noop, fmt.Errorf("building signer: %w", err)
	}
	if err := prepareSigner(ctx, s); err != nil {
		_ = s.Close() //nolint:errcheck
		return nil, noop, err
	}

	closer := func() {
		if err := s.Close(); err != nil {
			logrus.Warnf("closing signer credentials: %v", err)
		}
	}
	return append(opts, drop.WithSigner(s)), closer, nil
}

// prepareSigner obtains the sigstore signing credentials ahead of the first
// signature. Other backends need no preparation.
func prepareSigner(ctx context.Context, s *signer.Signer) error {
	if s.Options.Backend != signerOpts.BackendSigstore {
		return nil
	}
	if err := s.Options.Validate(); err != nil {
		return fmt.Errorf("validating signer options: %w", err)
	}
	creds := s.Options.BuildSigstoreCredentials()
	if err := creds.Prepare(ctx); err != nil {
		return fmt.Errorf("obtaining signing credentials: %w", err)
	}
	s.Credentials = creds
	return nil
}
