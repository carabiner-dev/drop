// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package drop

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	ampel "github.com/carabiner-dev/ampel/pkg/api/v1"
	"github.com/carabiner-dev/ampel/pkg/attestation"
	"github.com/carabiner-dev/ampel/pkg/collector"
	"github.com/carabiner-dev/ampel/pkg/policy"
	gitcollector "github.com/carabiner-dev/ampel/pkg/repository/git"
	"github.com/carabiner-dev/ampel/pkg/repository/release"
	"github.com/carabiner-dev/ampel/pkg/verifier"
	"github.com/carabiner-dev/hasher"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/release-utils/http"
	"sigs.k8s.io/release-utils/util"

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
	DownloadAssetToTmp(*GetOptions, github.AssetDataProvider) (string, error)

	// DownloadAssetToWriter gets an asset from a release to an already opened file
	DownloadAssetToWriter(*GetOptions, io.Writer, github.AssetDataProvider) error

	// DownloadAssetToWriter gets an asset from a release to an already opened file
	DownloadAssetToFile(*GetOptions, github.AssetDataProvider) (string, error)

	// VerifyAsset verifies that a file complioes with a set of policies
	VerifyAsset(*Options, []*ampel.PolicySet, github.AssetDataProvider, string) (bool, *ampel.ResultSet, error)

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
	// .. unless the asset spec has a name defined
	if spec.GetName() != "" {
		name = spec.GetName()
	}

	for _, asset := range assets {
		if asset.GetName() != name {
			continue
		}
		// Found. Now check if it has variants for the local OS
		if installable, ok := asset.(*github.Installable); ok {
			var wantedVariant github.AssetDataProvider
			sysPackageFormat := system.GetPreferredPackage(system.GetSystemOSFamily())

			for _, variant := range installable.Variants {
				// If the os or arch is not what we want, ignore it.
				if variant.Os != opts.OS || variant.Arch != opts.Arch {
					continue
				}

				// Check to see if its a package or archive
				packageType := system.PackageExtensions.GetTypeFromFile(variant.GetName())
				archiveType := system.ArchiveExtensions.GetTypeFromFile(variant.GetName())

				// If we want a binary and this is a package or archive, ignore
				if opts.DownloadType != "" && opts.DownloadType == "b" && (archiveType != "" || packageType != "") {
					continue
				}

				// If we want a package and this is not one, ignore
				if opts.DownloadType != "" && opts.DownloadType == "p" && packageType == "" {
					continue
				}

				// Same for archive, skip if it is something else
				if opts.DownloadType != "" && opts.DownloadType == "a" && archiveType == "" {
					continue
				}

				// For expected binaries, we use the installer name
				if packageType == "" && archiveType == "" {
					opts.computedFilename = installable.GetName()
					if variant.Os == system.OSWindows {
						opts.computedFilename += ".exe"
					}
				} else {
					// When handling packages or archives, keep the same name
					opts.computedFilename = variant.GetName()
				}

				// If we are not looking for a specific type, and its a
				// binary, return it immediately
				if packageType == "" && archiveType == "" {
					return variant, nil
				}

				// If we are looking for a package, check if the asset matches
				// the system format:
				if opts.DownloadType == "p" && packageType == sysPackageFormat {
					return variant, nil
				}

				// Otherwise capture the asset but prefer archives
				if wantedVariant == nil {
					wantedVariant = variant
				} else if archiveType != "" {
					wantedVariant = variant
				}
			}

			if wantedVariant != nil {
				opts.computedFilename = wantedVariant.GetName()
				return wantedVariant, nil
			}

			logrus.Debugf("no variant found for %s/%s", opts.OS, opts.Arch)
			return nil, ErrNoPlatformVariant
		}

		// If the file matches, it is not an installable, return the plain asset
		opts.computedFilename = asset.GetName()
		return asset, nil
	}

	// Before we go, now check all variants for a matching filename in case
	// the user specified the exact name in the URL spec:
	for _, asset := range assets {
		installable, ok := asset.(*github.Installable)
		if !ok {
			continue
		}
		for _, v := range installable.Variants {
			if v.GetName() == name {
				opts.computedFilename = v.GetName()
				return v, nil
			}
		}
	}

	return nil, fmt.Errorf("no asset found for %s", spec.GetRepo())
}

// FetchPolicies reads the artifact policies from the specified repo
func (di *defaultImplementation) FetchPolicies(opts *Options, asset github.AssetDataProvider) ([]*ampel.PolicySet, error) {
	repoBaseUrl := fmt.Sprintf(
		"https://%s/%s/%s", asset.GetHost(), asset.GetOrg(), defaultPolicyRepo,
	)
	if opts.PolicyRepository != "" {
		repoBaseUrl = opts.PolicyRepository
	}

	opts.Listener.HandleEvent(
		&Event{
			Object: EventObjectPolicy, Verb: EventVerbGet,
			Data: map[string]string{"repo": repoBaseUrl},
		},
	)

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
	ret := []*ampel.PolicySet{}
	parser := policy.NewParser()
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

	opts.Listener.HandleEvent(
		&Event{
			Object: EventObjectPolicy, Verb: EventVerbDone,
			Data: map[string]string{"count": fmt.Sprintf("%d", len(ret))},
		},
	)

	return ret, nil
}

// DownloadAssetToTmp fetches the asset to a temporary location
func (di *defaultImplementation) DownloadAssetToTmp(opts *GetOptions, asset github.AssetDataProvider) (string, error) {
	tmpfile, err := os.CreateTemp("", "drop-download-")
	if err != nil {
		return "", fmt.Errorf("creating temporary file: %w", err)
	}
	defer tmpfile.Close() //nolint:errcheck

	// Get the data
	if err := di.DownloadAssetToWriter(opts, tmpfile, asset); err != nil {
		return "", err
	}
	return tmpfile.Name(), nil
}

func (di *defaultImplementation) VerifyAsset(
	opts *Options, policies []*ampel.PolicySet, asset github.AssetDataProvider, filePath string,
) (bool, *ampel.ResultSet, error) {
	// Create a verifier, for now we will only support attestations
	// published along the artifact (as GitHub assets):

	opts.Listener.HandleEvent(
		&Event{Object: EventObjectVerification, Verb: EventVerbRunning},
	)

	// Create the collector
	clctr, err := release.New(
		release.WithRepo(asset.GetRepoURL()),
		release.WithTag(asset.GetVersion()),
	)
	if err != nil {
		return false, nil, fmt.Errorf("unable to create release attestation collector")
	}

	// Create the new ampel verifier
	vrfr, err := verifier.New(verifier.WithCollector(clctr))
	if err != nil {
		return false, nil, fmt.Errorf("creating new AMPEL verifier: %w", err)
	}

	// Generate the subject resource descriptors from the file
	res, err := hasher.New().HashFiles([]string{filePath})
	if err != nil {
		return false, nil, fmt.Errorf("hashing file: %w", err)
	}
	if len(*res) != 1 {
		return false, nil, fmt.Errorf("expected one set of hashes from file, got %d", len(*res))
	}

	// Run the artifact verification
	results, err := vrfr.Verify(
		context.Background(), &verifier.DefaultVerificationOptions, policies, res.ToResourceDescriptors()[0],
	)
	if err != nil {
		return false, nil, fmt.Errorf("error running artifact verification: %w", err)
	}

	// Compute the evaluation status
	passed := true
	for _, r := range results.GetResults() {
		if r.GetStatus() != ampel.StatusPASS {
			passed = false
		}
	}

	p := "true"
	if !passed {
		p = "false"
	}

	opts.Listener.HandleEvent(
		&Event{
			Object: EventObjectVerification, Verb: EventVerbDone,
			Data: map[string]string{"passed": p},
		},
	)

	return passed, results, nil
}

func (di *defaultImplementation) InstallAsset(*Options, *system.Info, string) error {
	return nil
}

// DownloadAssetToWriter downloads the asset data to the supplied writer
func (di *defaultImplementation) DownloadAssetToWriter(opts *GetOptions, w io.Writer, asset github.AssetDataProvider) error {
	if asset.GetDownloadURL() == "" {
		return fmt.Errorf("asset has nor download URL defined")
	}
	agent := http.NewAgent().WithTimeout(time.Duration(opts.TransferTimeOut) * time.Second)
	if err := agent.GetToWriter(w, asset.GetDownloadURL()); err != nil {
		return fmt.Errorf("fetching data: %w", err)
	}
	return nil
}

// DownloadAssetToFile downloads an asset to a file. The filename will be determined
// by the installable name, type and arch.
func (di *defaultImplementation) DownloadAssetToFile(opts *GetOptions, asset github.AssetDataProvider) (string, error) {
	filename := opts.computedFilename
	if opts.FileName != "" {
		// TODO(puerco): Check if this is a dir.
		//  and if so, use the computed filename and
		if util.IsDir(opts.FileName) {
			filename = path.Join(filename, opts.computedFilename)
		} else {
			filename = opts.FileName
		}
	}

	// Send the event to the notifier
	opts.Listener.HandleEvent(
		&Event{
			Object: EventObjectAsset, Verb: EventVerbGet,
			Data: map[string]string{"filename": filename, "size": fmt.Sprintf("%d", asset.GetSize())},
		},
	)

	p := filepath.Join(opts.DownloadPath, filename)
	if util.Exists(p) {
		return "", fmt.Errorf("file %q already exists, will not overwrite", p)
	}
	f, err := os.Create(p) //nolint:gosec
	if err != nil {
		return "", fmt.Errorf("downloading file: %w", err)
	}
	defer f.Close() //nolint:errcheck
	if err := di.DownloadAssetToWriter(opts, f, asset); err != nil {
		return "", err
	}

	return p, nil
}
