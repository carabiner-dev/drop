// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package drop

import (
	"errors"
	"fmt"
	"os"

	"github.com/carabiner-dev/drop/pkg/github"
)

const defaultPolicyRepo = ".ampel"

var (
	ErrNoPolicyAvailable  = errors.New("no verification policies available for artifact")
	ErrVerificationFailed = errors.New("asset failed verification, refusing to install")
	ErrNoPlatformVariant  = errors.New("no installable variant found for the specified platform")
)

type Dropper struct {
	Options Options
	client  *github.Client
	impl    installerImplementation
}

func New(funcs ...FuncOption) (*Dropper, error) {
	opts := defaultOptions

	// Create github client
	client, err := github.New()
	if err != nil {
		return nil, fmt.Errorf("creating github client: %w", err)
	}

	d := &Dropper{
		Options: opts,
		client:  client,
		impl:    &defaultImplementation{},
	}

	for _, fn := range funcs {
		if err := fn(d); err != nil {
			return nil, err
		}
	}
	return d, nil
}

// Get downloads and verifies an artifact from a GitHub release.
func (dropper *Dropper) Get(spec github.AssetDataProvider, funcs ...FuncGetOption) error {
	opts := defaultGetOptions
	opts.Options = dropper.Options

	for _, fn := range funcs {
		if err := fn(&opts); err != nil {
			return err
		}
	}

	// Ensure we have the noop listener so we don't have to check everytime
	if opts.Listener == nil {
		opts.Listener = &NoopListener{}
	}

	asset, err := dropper.impl.ChooseAsset(&opts, dropper.client, spec)
	if err != nil {
		return fmt.Errorf("unable to locate a suitable asset: %w", err)
	}

	// Look for the asset polcies
	policies, err := dropper.impl.FetchPolicies(&dropper.Options, asset)
	if err != nil {
		return fmt.Errorf("finding asset polcies: %w", err)
	}

	if len(policies) == 0 && !opts.SkipVerification {
		return ErrNoPolicyAvailable
	}

	downloadPath, err := dropper.impl.DownloadAssetToFile(&opts, asset)
	if err != nil {
		return fmt.Errorf("downloading file: %w", err)
	}

	// Verify the asset data
	if opts.SkipVerification {
		opts.Listener.HandleEvent(
			&Event{Object: EventObjectVerification, Verb: EventVerbSkipped},
		)
	} else {
		ok, _, err := dropper.impl.VerifyAsset(&dropper.Options, policies, asset, downloadPath)
		if err != nil {
			_ = os.Remove(downloadPath) //nolint:errcheck
			return fmt.Errorf("error verifying asset: %w", err)
		}

		// If verification failed, we're done
		if !ok {
			_ = os.Remove(downloadPath) //nolint:errcheck
			return ErrVerificationFailed
		}
	}

	opts.Listener.HandleEvent(
		&Event{
			Object: EventObjectAsset, Verb: EventVerbSaved,
			Data: map[string]string{"path": downloadPath},
		},
	)

	return nil
}

// Install downloads, verifies and installs an artifact from a release
func (dropper *Dropper) Install(spec github.AssetDataProvider, funcs ...FuncGetOption) error {
	opts := defaultGetOptions
	opts.Options = dropper.Options

	for _, fn := range funcs {
		if err := fn(&opts); err != nil {
			return err
		}
	}

	// Ensure we have the noop listener so we don't have to check everytime
	if opts.Listener == nil {
		opts.Listener = &NoopListener{}
	}

	sysinfo, err := dropper.impl.GetSystemInfo(&dropper.Options)
	if err != nil {
		return fmt.Errorf("reading system information: %w", err)
	}

	asset, err := dropper.impl.ChooseAsset(&opts, dropper.client, spec)
	if err != nil {
		return fmt.Errorf("unable to locate a suitable asset: %w", err)
	}

	// Look for the asset polcies
	policies, err := dropper.impl.FetchPolicies(&dropper.Options, asset)
	if err != nil {
		return fmt.Errorf("finding asset polcies: %w", err)
	}

	// Downlad the asset to install
	downloadPath, err := dropper.impl.DownloadAssetToTmp(&opts, asset)
	if err != nil {
		return fmt.Errorf("downloading asset: %w", err)
	}

	// Verify the asset data
	ok, _, err := dropper.impl.VerifyAsset(&dropper.Options, policies, asset, downloadPath)
	if err != nil {
		return fmt.Errorf("error verifying asset: %w", err)
	}

	// If verification failed, we're done
	if !ok {
		return ErrVerificationFailed
	}

	// TODO(puerco): Probably here we should output a summary of the verification

	// Install the asset in the system
	if err := dropper.impl.InstallAsset(&dropper.Options, sysinfo, downloadPath); err != nil {
		return fmt.Errorf("installing asset: %w", err)
	}

	return nil
}
