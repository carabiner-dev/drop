// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package drop

import (
	"errors"
	"fmt"

	"github.com/carabiner-dev/drop/pkg/github"
)

var ErrVerificationFailed = errors.New("asset failed verification, refusing to install")

type Installer struct {
	Options Options
	client  github.Client
	impl    installerImplementation
}

type Options struct {
}

// Install downloads, verifies and installs an artifact from a release
func (installer *Installer) Install(spec github.AssetDataProvider) error {
	sysinfo, err := installer.impl.GetSystemInfo(&installer.Options)
	if err != nil {
		return fmt.Errorf("reading system information: %w", err)
	}

	asset, err := installer.impl.ChooseAsset(&installer.Options, spec, sysinfo)
	if err != nil {
		return fmt.Errorf("unable to locate a suitable asset: %w", err)
	}

	// Look for the asset polcies
	policies, err := installer.impl.FetchPolicies(&installer.Options, asset)
	if err != nil {
		return fmt.Errorf("finding asset polcies: %w", err)
	}

	// Downlad the asset to install
	downloadPath, err := installer.impl.DownloadAssetToTmp(&installer.Options, asset)
	if err != nil {
		return fmt.Errorf("downloading asset: %w", err)
	}

	// Verify the asset data
	ok, err := installer.impl.VerifyAsset(&installer.Options, policies, asset, downloadPath)
	if err != nil {
		return fmt.Errorf("error verifying asset: %w", err)
	}

	// If verification failed, we're done
	if !ok {
		return ErrVerificationFailed
	}

	// TODO(puerco): Probably here we should output a summary of the verification

	// Install the asset in the system
	if err := installer.impl.InstallAsset(&installer.Options, sysinfo, downloadPath); err != nil {
		return fmt.Errorf("installing asset: %w", err)
	}

	return nil
}
