// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package drop

import (
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
func (di *defaultImplementation) DownloadAssetToTmp(*Options, github.AssetDataProvider) (string, error) {
	return "", nil
}
func (di *defaultImplementation) VerifyAsset(*Options, []ampel.PolicySet, github.AssetDataProvider, string) (bool, error) {
	return false, nil
}
func (di *defaultImplementation) InstallAsset(*Options, *system.Info, string) error {
	return nil
}
