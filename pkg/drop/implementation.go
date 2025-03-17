// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package drop

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/release-utils/http"
	"sigs.k8s.io/release-utils/util"

	ampel "github.com/carabiner-dev/ampel/pkg/api/v1"
	"github.com/carabiner-dev/ampel/pkg/attestation"
	"github.com/carabiner-dev/ampel/pkg/collector"
	"github.com/carabiner-dev/ampel/pkg/policy"
	gitcollector "github.com/carabiner-dev/ampel/pkg/repository/git"
	"github.com/sirupsen/logrus"

	"github.com/carabiner-dev/drop/pkg/github"
	"github.com/carabiner-dev/drop/pkg/system"
)

type installerImplementation interface {
	// GetSystemInfo reads the required data from the system to let the installer
	// choose the proper artifacts and how and where to install binaries and packages.
	GetSystemInfo(*Options) (*system.Info, error)

	// Choose asset takes an asset specifier and chooses the proper file to download
	// and install in the system.
	ChooseAsset(*GetOptions, *github.Client, github.AssetDataProvider) (github.AssetDataProvider, error)

	// Fetch policies uses a provider to look for policies in a structured data source.
	FetchPolicies(*Options, github.AssetDataProvider) ([]*ampel.PolicySet, error)

	// Download asset gets a file from a github release and makes it available in a directory
	DownloadAssetToTmp(*Options, github.AssetDataProvider) (string, error)

	// DownloadAssetToWriter gets an asset from a release to an already opened file
	DownloadAssetToWriter(*Options, io.Writer, github.AssetDataProvider) error

	// DownloadAssetToWriter gets an asset from a release to an already opened file
	DownloadAssetToFile(*GetOptions, github.AssetDataProvider) error

	// VerifyAsset verifies that a file complioes with a set of policies
	VerifyAsset(*Options, []*ampel.PolicySet, github.AssetDataProvider, string) (bool, error)

	// InstallAsset invokes the system mechanism to set up the downloaded artifact
	// in the local machine.
	InstallAsset(*Options, *system.Info, string) error
}

type defaultImplementation struct{}

func (di *defaultImplementation) GetSystemInfo(*Options) (*system.Info, error) {
	return system.GetInfo()
}

// ChooseAsset selects an installable matching the spec name and local platform
func (di *defaultImplementation) ChooseAsset(opts *GetOptions, client *github.Client, spec github.AssetDataProvider) (github.AssetDataProvider, error) {
	assets, err := client.ListReleaseInstallables(spec)
	if err != nil {
		return nil, fmt.Errorf("fetching release assets: %w", err)
	}

	// We look a for an installable with the same name as the repo
	name := spec.GetRepo()
	// .. unless the asset get has a name
	if spec.GetName() != "" {
		name = spec.GetName()
	}
	for _, asset := range assets {
		if asset.GetName() == name {
			// Found. Now check if it has variants for the local OS
			if installable, ok := asset.(*github.Installable); ok {
				for _, variant := range installable.Variants {
					if variant.Os == opts.OS && variant.Arch == opts.Arch {
						opts.FileName = installable.GetName()
						return variant, nil
					}
				}
				logrus.Debugf("no variant found for %s/%s", opts.OS, opts.Arch)
				return nil, ErrNoPlatformVariant
			}
		}
	}
	return nil, fmt.Errorf("no asset found for %s", spec.GetRepo())
}

func (di *defaultImplementation) FetchPolicies(opts *Options, asset github.AssetDataProvider) ([]*ampel.PolicySet, error) {
	repoBaseUrl := fmt.Sprintf(
		"https://%s/%s/%s", asset.GetHost(), asset.GetOrg(), defaultPolicyRepo,
	)
	if opts.PolicyRepository != "" {
		repoBaseUrl = opts.PolicyRepository
	}

	locator := fmt.Sprintf(
		"%s#policy/%s/%s/%s", repoBaseUrl,
		asset.GetHost(), asset.GetOrg(), asset.GetRepo(),
	)

	logrus.Debugf("Fetching policies from %s", locator)

	// Create the git repository for the collector agent
	arepo, err := gitcollector.New(
		gitcollector.WithLocator(locator),
	)
	if err != nil {
		return nil, fmt.Errorf("creating git collector: %w", err)
	}
	// Create the attestation fetcher
	agent, err := collector.New(
		collector.WithRepository(arepo),
	)
	if err != nil {
		return nil, fmt.Errorf("creating collector agent: %w", err)
	}

	// Now, fetch all policy attestations
	attestations, err := agent.FetchAttestationsByPredicateType(
		context.Background(), []attestation.PredicateType{"https://carabiner.dev/ampel/policyset/v0.0.1"},
	)
	// If there were errors fetching attestations, there are two special
	// cases we want to handle as non-errors:
	if err != nil {
		// 1. The org has no ampel repository.
		// This error also returns if the requires auth
		if strings.Contains(err.Error(), "Repository not found") {
			logrus.Debugf("policy repository does not exist")
			return []*ampel.PolicySet{}, nil
		}

		// 2. The policy repo exists, but the specified path does not exist.
		if strings.Contains(err.Error(), "file does not exist") {
			logrus.Debug("policy repository has no policies for repo")
			return []*ampel.PolicySet{}, nil
		}

		// Otherwise it is a true error
		return nil, fmt.Errorf("fetching policies: %w", err)
	}

	// Parse the policies from the attested data
	var ret = []*ampel.PolicySet{}
	var parser = policy.NewParser()
	for _, att := range attestations {
		// Since these attestations were already parsed, these two
		// should never happen, but we still want to avoid panics:
		if att.GetStatement() == nil {
			logrus.Error("policy attestation has no statement")
			continue
		}
		if att.GetStatement().GetPredicate() == nil {
			logrus.Error("policy attestation has no predicate")
			continue
		}
		pset, err := parser.ParseSet(att.GetStatement().GetPredicate().GetData())
		if err != nil {
			logrus.Error("parsing policy set: %w", err)
			continue
		}
		ret = append(ret, pset)
	}
	return ret, nil
}

// DownloadAssetToTmp fetches the asset to a temporary location
func (di *defaultImplementation) DownloadAssetToTmp(_ *Options, asset github.AssetDataProvider) (string, error) {
	tmpfile, err := os.CreateTemp("", "drop-download-")
	if err != nil {
		return "", fmt.Errorf("creating temporary file: %w", err)
	}
	defer tmpfile.Close()

	// Get the data
	if err := di.DownloadAssetToWriter(nil, tmpfile, asset); err != nil {
		return "", err
	}
	return tmpfile.Name(), nil
}
func (di *defaultImplementation) VerifyAsset(*Options, []*ampel.PolicySet, github.AssetDataProvider, string) (bool, error) {
	return false, nil
}
func (di *defaultImplementation) InstallAsset(*Options, *system.Info, string) error {
	return nil
}

// DownloadAssetToWriter downloads the asset data to the supplied writer
func (di *defaultImplementation) DownloadAssetToWriter(_ *Options, w io.Writer, asset github.AssetDataProvider) error {
	if asset.GetDownloadURL() == "" {
		return fmt.Errorf("asset has nor download URL defined")
	}
	agent := http.NewAgent()
	if err := agent.GetToWriter(w, asset.GetDownloadURL()); err != nil {
		return fmt.Errorf("fetching data: %w", err)
	}
	return nil
}

func (di *defaultImplementation) DownloadAssetToFile(opts *GetOptions, asset github.AssetDataProvider) error {
	filename := asset.GetName()
	if opts.FileName != "" {
		filename = opts.FileName
	}

	// FIXME(puerco): This should no be done when getting packages or
	// archives:
	if opts.OS == system.OSWindows {
		filename = filename + ".exe"
	}
	path := filepath.Join(opts.DownloadPath, filename)
	if util.Exists(path) {
		return fmt.Errorf("file %q already exists, will not overwrite", path)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("downloading file: %w", err)
	}

	return di.DownloadAssetToWriter(nil, f, asset)
}
