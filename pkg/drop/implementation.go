// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package drop

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"sigs.k8s.io/release-utils/http"

	ampel "github.com/carabiner-dev/ampel/pkg/api/v1"
	"github.com/carabiner-dev/ampel/pkg/attestation"
	"github.com/carabiner-dev/ampel/pkg/collector"
	"github.com/carabiner-dev/ampel/pkg/policy"
	gitcollector "github.com/carabiner-dev/ampel/pkg/repository/git"
	"github.com/go-git/go-git/v5/plumbing/transport"
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
	ChooseAsset(*Options, github.AssetDataProvider, *system.Info) (github.AssetDataProvider, error)

	// Fetch policies uses a provider to look for policies in a structured data source.
	FetchPolicies(*Options, github.AssetDataProvider) ([]*ampel.PolicySet, error)

	// Download asset gets a file from a github release and makes it available in a directory
	DownloadAssetToTmp(*Options, github.AssetDataProvider) (string, error)

	// DownloadAssetToWriter gets an asset from a release to an already opened file
	DownloadAssetToWriter(*Options, io.Writer, github.AssetDataProvider) error

	// DownloadAssetToWriter gets an asset from a release to an already opened file
	DownloadAssetToFile(*Options, string, github.AssetDataProvider) error

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
func (di *defaultImplementation) ChooseAsset(*Options, github.AssetDataProvider, *system.Info) (github.AssetDataProvider, error) {
	return nil, nil
}

func (di *defaultImplementation) FetchPolicies(opts *Options, asset github.AssetDataProvider) ([]*ampel.PolicySet, error) {
	repoBaseUrl := fmt.Sprintf(
		"git+https://%s/%s/%s", asset.GetHost(), asset.GetOrg(), defaultPolicyRepo,
	)
	if opts.PolicyRepository != "" {
		repoBaseUrl = opts.PolicyRepository
	}

	// Create the git repository for the collector agent
	arepo, err := gitcollector.New(
		gitcollector.WithLocator(
			fmt.Sprintf(
				"%s#policy/%s/%s/%s", repoBaseUrl,
				asset.GetHost(), asset.GetOrg(), asset.GetRepo(),
			),
		),
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
		context.Background(), []attestation.PredicateType{"https://carabiner.dev/ampel/results/v0.0.1"},
	)
	// If there were errors fetching attestations, there are two special
	// cases we want to handle as non-errors:
	if err != nil {
		// 1. The org has no ampel repository.
		// This error also returns if the requires auth
		if strings.Contains(err.Error(), transport.ErrRepositoryNotFound.Error()) {
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
func (di *defaultImplementation) VerifyAsset(*Options, []*ampel.PolicySet, github.AssetDataProvider, string) (bool, error) {
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

func (di *defaultImplementation) DownloadAssetToFile(opts *Options, path string, asset github.AssetDataProvider) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("downloading file: %w", err)
	}

	return di.DownloadAssetToWriter(opts, f, asset)
}
