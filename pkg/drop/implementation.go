// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package drop

import (
	"fmt"
	"io"
	"os"

	"sigs.k8s.io/release-utils/http"

	ampel "github.com/carabiner-dev/ampel/pkg/api/v1"

	"github.com/carabiner-dev/drop/pkg/github"
	"github.com/carabiner-dev/drop/pkg/system"
)

type installerImplementation interface {
	// GetSystemInfo reads the required data from the system to let the installer
	// choose the proper artifacts and how and where to install binaries and packages.
	GetSystemInfo(*Options) (*system.Info, error)

	// Choose asset takes an asset specifier and chooses the proper file to download
	// and install in the system.
	ChooseAsset(*Options, github.AssetDataProvider, *system.Info) (github.AssetDataProvider, error)

	// Fetch policies uses a provider to look for policies in a structured data source.
	FetchPolicies(*Options, github.AssetDataProvider) ([]ampel.PolicySet, error)

	// Download asset gets a file from a github release and makes it available in a directory
	DownloadAssetToTmp(*Options, github.AssetDataProvider) (string, error)

	// DownloadAssetToWriter gets an asset from a release to an already opened file
	DownloadAssetToWriter(*Options, io.Writer, github.AssetDataProvider) error

	// VerifyAsset verifies that a file complioes with a set of policies
	VerifyAsset(*Options, []ampel.PolicySet, github.AssetDataProvider, string) (bool, error)

	// InstallAsset invokes the system mechanism to set up the downloaded artifact
	// in the local machine.
	InstallAsset(*Options, *system.Info, string) error
}

type defaultImplementation struct{}

func (di *defaultImplementation) GetSystemInfo(*Options) (*system.Info, error) {
	return system.GetInfo()
}
func (di *defaultImplementation) ChooseAsset(*Options, github.AssetDataProvider, *system.Info) (github.AssetDataProvider, error) {
	return nil, nil
}
func (di *defaultImplementation) FetchPolicies(*Options, github.AssetDataProvider) ([]ampel.PolicySet, error) {
	return nil, nil
}

// DownloadAssetToTmp fetches the asset to a temporary location
func (di *defaultImplementation) DownloadAssetToTmp(opts *Options, asset github.AssetDataProvider) (string, error) {
	tmpfile, err := os.CreateTemp("", "drop-download-")
	if err != nil {
		return "", fmt.Errorf("creating temporary file: %w", err)
	}
	defer tmpfile.Close()

	// Get the data
	if err := di.DownloadAssetToWriter(opts, tmpfile, asset); err != nil {
		return "", err
	}
	return tmpfile.Name(), nil
}
func (di *defaultImplementation) VerifyAsset(*Options, []ampel.PolicySet, github.AssetDataProvider, string) (bool, error) {
	return false, nil
}
func (di *defaultImplementation) InstallAsset(*Options, *system.Info, string) error {
	return nil
}

// DownloadAssetToWriter downloads the asset data to the supplied writer
func (di *defaultImplementation) DownloadAssetToWriter(opts *Options, w io.Writer, asset github.AssetDataProvider) error {
	if asset.GetDownloadURL() == "" {
		return fmt.Errorf("asset has nor download URL defined")
	}
	agent := http.NewAgent()
	if err := agent.GetToWriter(w, asset.GetDownloadURL()); err != nil {
		return fmt.Errorf("fetching data: %w", err)
	}
	return nil
}
