// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package drop

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/carabiner-dev/ampel/pkg/verifier"
	"github.com/carabiner-dev/attestation"
	"github.com/carabiner-dev/collector"
	fscollector "github.com/carabiner-dev/collector/repository/filesystem"
	gitcollector "github.com/carabiner-dev/collector/repository/git"
	"github.com/carabiner-dev/collector/repository/release"
	"github.com/carabiner-dev/hasher"
	"github.com/carabiner-dev/policy"
	papi "github.com/carabiner-dev/policy/api/v1"
	"github.com/carabiner-dev/predicates"
	"github.com/go-git/go-billy/v5/helper/iofs"
	intoto "github.com/in-toto/attestation/go/v1"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"
	util "sigs.k8s.io/release-utils/helpers"
	"sigs.k8s.io/release-utils/http"

	"github.com/carabiner-dev/drop/pkg/github"
	"github.com/carabiner-dev/drop/pkg/inventory"
	"github.com/carabiner-dev/drop/pkg/system"
)

type installerImplementation interface {
	// GetSystemInfo reads the required data from the system to let the installer
	// choose the proper artifacts and how and where to install binaries and packages.
	GetSystemInfo(*Options) (*system.Info, error)

	// Choose asset takes an asset specifier and chooses the proper file to download
	// and install in the system.
	ChooseAsset(*GetOptions, *github.Client, github.AssetDataProvider) (github.AssetDataProvider, error)

	// SelectInstallArtifact decides which release artifact (binary or system
	// package) will be installed on the local system.
	SelectInstallArtifact(*GetOptions, *github.Client, *system.Info, github.AssetDataProvider) (*InstallArtifact, error)

	// Fetch policies uses a provider to look for policies in a structured data source.
	FetchPolicies(*Options, github.AssetDataProvider) ([]*papi.PolicySet, error)

	// Download asset gets a file from a github release and makes it available in a directory
	DownloadAssetToTmp(*GetOptions, github.AssetDataProvider) (string, error)

	// DownloadAssetToWriter gets an asset from a release to an already opened file
	DownloadAssetToWriter(*GetOptions, io.Writer, github.AssetDataProvider) error

	// DownloadAssetToWriter gets an asset from a release to an already opened file
	DownloadAssetToFile(*GetOptions, github.AssetDataProvider) (string, error)

	// VerifyAsset verifies that a file complioes with a set of policies
	VerifyAsset(*Options, []*papi.PolicySet, github.AssetDataProvider, string) (bool, *papi.ResultSet, error)

	// AttestResults writes an attestation of a verification of the named
	// app's asset and returns the path of the file.
	AttestResults(*GetOptions, string, github.AssetDataProvider, *papi.ResultSet) (string, error)

	// InstallAsset invokes the system mechanism to set up the downloaded artifact
	// in the local machine.
	InstallAsset(*GetOptions, *system.Info, *InstallArtifact, string) error

	// RecordInstall registers a successful installation in the user's
	// inventory database so it can later be verified, updated or removed.
	RecordInstall(*GetOptions, *InstallArtifact, string, bool) error

	// RemoveInstalled removes the artifact a previous installation left in
	// the system: a binary file or a package (through the package manager).
	RemoveInstalled(*GetOptions, *inventory.Record) error
}

type defaultImplementation struct {
	runner commandRunner

	// inventoryPath overrides the location of the inventory database,
	// when empty the default (in the user's config dir) is used.
	inventoryPath string

	// policyRepository overrides how the default policy repository of an
	// organization is located (tests point it at local repositories).
	// When nil, DefaultPolicyRepository is used.
	policyRepository func(host, org string) string
}

func (di *defaultImplementation) GetSystemInfo(*Options) (*system.Info, error) {
	return system.GetInfo()
}

// findInstallable looks in a list of release assets for the installable (or
// plain asset) matching the spec name, defaulting to the repository name.
// findInstallable returns the installable (or plain asset) a spec points to.
// The spec name, defaulting to the repository name, is matched first. When
// the spec carries no name and nothing in the release is named after the
// repository, the fallback is the only installable shipping a variant for
// the platform (releases often name binaries differently than the repo).
// Several such installables are an error listing them, so the user can pick
// one with the #name syntax. Nothing matching returns nil without error.
func findInstallable(assets []github.AssetDataProvider, spec github.AssetDataProvider, osName, arch string) (github.AssetDataProvider, error) {
	name := specName(spec)
	for _, asset := range assets {
		if asset.GetName() == name {
			return asset, nil
		}
	}

	// An explicit name is never second-guessed
	if spec.GetName() != "" {
		return nil, nil
	}

	candidates := []string{}
	var found github.AssetDataProvider
	for _, asset := range assets {
		inst, ok := asset.(*github.Installable)
		if !ok || !hasVariant(inst, osName, arch) {
			continue
		}
		candidates = append(candidates, inst.GetName())
		found = inst
	}
	switch len(candidates) {
	case 0:
		return nil, nil
	case 1:
		logrus.Debugf("no asset named %q, using the only installable for %s/%s: %s", name, osName, arch, candidates[0])
		return found, nil
	default:
		return nil, fmt.Errorf(
			"%w for %s/%s (%s): pick one with %s/%s/%s#<name>", ErrAmbiguousInstallable,
			osName, arch, strings.Join(candidates, ", "), spec.GetHost(), spec.GetOrg(), spec.GetRepo(),
		)
	}
}

// hasVariant reports if an installable ships a variant for a platform.
func hasVariant(inst *github.Installable, osName, arch string) bool {
	for _, v := range inst.Variants {
		if v.Os == osName && v.Arch == arch {
			return true
		}
	}
	return false
}

// ChooseAsset selects an installable matching the spec name and local platform
func (di *defaultImplementation) ChooseAsset(opts *GetOptions, client *github.Client, spec github.AssetDataProvider) (github.AssetDataProvider, error) {
	assets, err := client.ListReleaseInstallables(spec)
	if err != nil {
		return nil, fmt.Errorf("fetching release assets: %w", err)
	}

	asset, err := findInstallable(assets, spec, opts.OS, opts.Arch)
	if err != nil {
		return nil, err
	}
	if asset != nil {
		// Found. Now check if it has variants for the local OS
		if installable, ok := asset.(*github.Installable); ok {
			var wantedVariant github.AssetDataProvider
			var binaryVariant *github.Asset
			sysPackageFormat := system.GetPreferredPackage(system.GetSystemOSFamily())

			for _, variant := range installable.Variants {
				// If the os or arch is not what we want, ignore it.
				if variant.Os != opts.OS || variant.Arch != opts.Arch {
					continue
				}

				// Signatures, SBOMs and other metadata files published
				// along the artifacts are never chosen automatically.
				if isMetadataFile(variant.GetName()) {
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

				// Binaries win over packages and archives, but keep
				// looking in case the release ships several flavors for
				// the platform: the canonical one (shortest name) wins.
				if packageType == "" && archiveType == "" {
					if binaryVariant == nil || len(variant.GetName()) < len(binaryVariant.GetName()) {
						binaryVariant = variant
					}
					continue
				}

				// If we are looking for a package, check if the asset matches
				// the system format:
				if opts.DownloadType == "p" && packageType == sysPackageFormat {
					opts.computedFilename = variant.GetName()
					return variant, nil
				}

				// Otherwise capture the asset but prefer archives
				if wantedVariant == nil {
					wantedVariant = variant
				} else if archiveType != "" {
					wantedVariant = variant
				}
			}

			// Binaries are downloaded under the installable name
			if binaryVariant != nil {
				opts.computedFilename = installable.GetName()
				if binaryVariant.Os == system.OSWindows {
					opts.computedFilename += ".exe"
				}
				return binaryVariant, nil
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
	name := specName(spec)
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

// FetchPolicies reads the policies that apply to an artifact: those its
// organization publishes (or the ones in the configured policy repository)
// and, when there are none, the community maintained ones.
func (di *defaultImplementation) FetchPolicies(opts *Options, asset github.AssetDataProvider) ([]*papi.PolicySet, error) {
	locate := di.policyRepository
	if locate == nil {
		locate = DefaultPolicyRepository
	}
	repoBaseUrl := locate(asset.GetHost(), asset.GetOrg())
	if opts.PolicyRepository != "" {
		repoBaseUrl = opts.PolicyRepository
	}

	// Policies are read from two directories of the policy repository:
	// the organization-wide one, applying to every release, and the
	// repository's own.
	opts.Listener.HandleEvent(
		&Event{
			Object: EventObjectPolicy, Verb: EventVerbGet,
			Data: map[string]string{"repo": repoBaseUrl, dataKeyPath: PolicyPathsLabel(asset.GetRepo())},
		},
	)
	sets, err := fetchPolicySets(repoBaseUrl, PolicyPaths(asset.GetRepo()))
	if err != nil {
		return nil, err
	}
	opts.Listener.HandleEvent(
		&Event{
			Object: EventObjectPolicy, Verb: EventVerbDone,
			Data: map[string]string{"count": fmt.Sprintf("%d", len(sets))},
		},
	)

	// Projects without policies of their own fall back to the community
	// repository, unless the policy source was set explicitly.
	if len(sets) > 0 || opts.PolicyRepository != "" || opts.CommunityPolicyRepository == "" {
		return sets, nil
	}

	communityPath := CommunityPolicyPath(asset.GetOrg(), asset.GetRepo())
	opts.Listener.HandleEvent(
		&Event{
			Object: EventObjectPolicy, Verb: EventVerbGet,
			Data: map[string]string{
				"repo": opts.CommunityPolicyRepository, dataKeyPath: communityPath + "/",
				EventDataCommunity: strconv.FormatBool(true),
			},
		},
	)
	sets, err = fetchPolicySets(opts.CommunityPolicyRepository, []string{communityPath})
	if err != nil {
		return nil, err
	}
	opts.Listener.HandleEvent(
		&Event{
			Object: EventObjectPolicy, Verb: EventVerbDone,
			Data: map[string]string{"count": fmt.Sprintf("%d", len(sets)), EventDataCommunity: strconv.FormatBool(true)},
		},
	)
	return sets, nil
}

// fetchPolicySets reads the policy set attestations found in the given
// directories of a git repository, all from a single shallow clone. A
// missing repository or directory yields no policy sets, not an error.
func fetchPolicySets(repoBaseUrl string, dirs []string) ([]*papi.PolicySet, error) {
	// Create the git repository for the collector agent
	arepo, err := gitcollector.New(
		gitcollector.WithLocator(fmt.Sprintf("%s#%s", repoBaseUrl, dirs[0])),
	)
	if err != nil {
		return nil, fmt.Errorf("creating git collector: %w", err)
	}

	ret := []*papi.PolicySet{}
	for i, dir := range dirs {
		if i > 0 {
			if err := setCollectorPath(arepo, dir); err != nil {
				return nil, err
			}
		}
		logrus.Debugf("Fetching policies from %s#%s", repoBaseUrl, dir)

		// Create the attestation fetcher. Agents cache what they fetch
		// by predicate type, so each directory gets its own agent over
		// the shared clone.
		agent, err := collector.New(
			collector.WithRepository(arepo),
		)
		if err != nil {
			return nil, fmt.Errorf("creating collector agent: %w", err)
		}

		// Fetch all policy set attestations, published under the current
		// predicate type or the legacy one.
		attestations, err := agent.FetchAttestationsByPredicateType(
			context.Background(), []attestation.PredicateType{
				predicates.PredicateTypePolicySet, predicates.PredicateTypePolicySet0,
			},
		)
		// If there were errors fetching attestations, there are two special
		// cases we want to handle as non-errors:
		if err != nil {
			// 1. The policy repository does not exist (this error also
			// returns if the repository requires auth). Nothing else to read.
			if strings.Contains(strings.ToLower(err.Error()), "repository not found") {
				logrus.Debugf("policy repository %s does not exist", repoBaseUrl)
				return []*papi.PolicySet{}, nil
			}

			// 2. The policy repo exists, but this directory does not.
			if strings.Contains(err.Error(), "file does not exist") {
				logrus.Debugf("policy repository has no policies in %s", dir)
				continue
			}

			// Otherwise it is a true error
			return nil, fmt.Errorf("fetching policies from %s: %w", dir, err)
		}
		ret = append(ret, parsePolicySets(attestations)...)
	}
	return ret, nil
}

// setCollectorPath points an already cloned git collector at another
// directory of the same checkout, so several policy directories are read
// from a single clone.
func setCollectorPath(c *gitcollector.Collector, dir string) error {
	if c.Repo == nil {
		return errors.New("policy repository is not cloned")
	}
	wt, err := c.Repo.Worktree()
	if err != nil {
		return fmt.Errorf("reading policy repository worktree: %w", err)
	}
	fsc, err := fscollector.New(
		fscollector.WithFS(iofs.New(wt.Filesystem)),
		fscollector.WithPath(dir),
	)
	if err != nil {
		return fmt.Errorf("creating filesystem collector: %w", err)
	}
	c.FSCollector = fsc
	return nil
}

// parsePolicySets parses the policy sets carried by policy attestations.
func parsePolicySets(attestations []attestation.Envelope) []*papi.PolicySet {
	ret := []*papi.PolicySet{}
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
		pset, err := parser.ParsePolicySet(att.GetStatement().GetPredicate().GetData())
		if err != nil {
			logrus.Errorf("parsing policy set: %v", err)
			continue
		}
		ret = append(ret, pset)
	}
	return ret
}

// DownloadAssetToTmp fetches the asset to a temporary directory, keeping its
// filename (package managers require local files to have proper extensions).
func (di *defaultImplementation) DownloadAssetToTmp(opts *GetOptions, asset github.AssetDataProvider) (string, error) {
	dir, err := os.MkdirTemp("", "drop-install-")
	if err != nil {
		return "", fmt.Errorf("creating temporary directory: %w", err)
	}

	filename := opts.computedFilename
	if filename == "" {
		filename = asset.GetName()
	}

	// Send the event to the notifier
	opts.Listener.HandleEvent(
		&Event{
			Object: EventObjectAsset, Verb: EventVerbGet,
			Data: map[string]string{"filename": filename, "size": fmt.Sprintf("%d", asset.GetSize())},
		},
	)

	filePath := filepath.Join(dir, filename)
	tmpfile, err := os.Create(filePath) //nolint:gosec
	if err != nil {
		_ = os.RemoveAll(dir) //nolint:errcheck
		return "", fmt.Errorf("creating temporary file: %w", err)
	}
	defer tmpfile.Close() //nolint:errcheck

	// Get the data
	if err := di.DownloadAssetToWriter(opts, tmpfile, asset); err != nil {
		_ = os.RemoveAll(dir) //nolint:errcheck
		return "", err
	}
	return filePath, nil
}

// assetSubject hashes the downloaded file into the subject descriptor of the
// verification. The descriptor names the release asset and points at its
// download URL rather than at the local copy, so the identity carried into
// policies and attestations is the published artifact, not a temporary file.
func assetSubject(asset github.AssetDataProvider, filePath string) (*intoto.ResourceDescriptor, error) {
	res, err := hasher.New().HashFiles([]string{filePath})
	if err != nil {
		return nil, fmt.Errorf("hashing file: %w", err)
	}
	if len(*res) != 1 {
		return nil, fmt.Errorf("expected one set of hashes from file, got %d", len(*res))
	}
	subject := res.ToResourceDescriptors()[0]
	if url := asset.GetDownloadURL(); url != "" {
		subject.Uri = url
	}
	if name := asset.GetName(); name != "" {
		subject.Name = name
	}
	return subject, nil
}

// finalizeResultSet fills in the set level fields the verifier leaves empty
// when evaluating a list of policy sets: the subject, the evaluation dates
// and the status. The status follows drop's own verdict (any result other
// than PASS fails the verification) so attestations built from the set
// never contradict what drop did. When a single policy set was evaluated
// it is referenced the way the verifier does when running one.
func finalizeResultSet(
	rs *papi.ResultSet, subject *intoto.ResourceDescriptor, policies []*papi.PolicySet,
	start *timestamppb.Timestamp, passed bool,
) {
	rs.Subject = subject
	rs.Status = papi.StatusFAIL
	if passed {
		rs.Status = papi.StatusPASS
	}
	if rs.GetDateStart() == nil {
		rs.DateStart = start
	}
	if rs.GetDateEnd() == nil {
		rs.DateEnd = timestamppb.Now()
	}
	if len(policies) == 1 && rs.GetPolicySet() == nil {
		rs.PolicySet = &papi.PolicyRef{
			Id:      policies[0].GetId(),
			Version: policies[0].GetMeta().GetVersion(),
		}
		rs.Meta = policies[0].GetMeta()
	}
}

func (di *defaultImplementation) VerifyAsset(
	opts *Options, policies []*papi.PolicySet, asset github.AssetDataProvider, filePath string,
) (bool, *papi.ResultSet, error) {
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

	// Generate the subject resource descriptor from the file
	subject, err := assetSubject(asset, filePath)
	if err != nil {
		return false, nil, err
	}

	// Run the artifact verification
	start := timestamppb.Now()
	results, err := vrfr.Verify(
		context.Background(), &verifier.DefaultVerificationOptions, policies, subject,
	)
	if err != nil {
		return false, nil, fmt.Errorf("error running artifact verification: %w", err)
	}

	resultSet, ok := results.(*papi.ResultSet)
	if !ok {
		return false, nil, fmt.Errorf("unexpected results type %T returned from verifier", results)
	}

	// Compute the evaluation status
	passed := true
	for _, r := range resultSet.GetResults() {
		if r.GetStatus() != papi.StatusPASS {
			passed = false
		}
	}
	finalizeResultSet(resultSet, subject, policies, start, passed)

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

	return passed, resultSet, nil
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
