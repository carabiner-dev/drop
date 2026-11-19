// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package drop

import (
	"fmt"
	"maps"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"

	"github.com/carabiner-dev/drop/pkg/github"
	"github.com/carabiner-dev/drop/pkg/inventory"
)

// UpdateStatus describes the update state of an app installed with drop.
type UpdateStatus struct {
	// Record is the inventory entry of the installed app.
	Record *inventory.Record

	// LatestVersion is the newest release available in the repository.
	LatestVersion string

	// UpdateAvailable is true when the latest release is newer than the
	// installed version.
	UpdateAvailable bool

	// Error captures a per-app check failure (API errors, repository
	// gone, etc) without aborting the rest of the checks.
	Error error
}

// CheckUpdates reads the inventory of installed apps and checks the GitHub
// releases of each of them to see if a newer version is available.
//
// When app references are passed, only the apps they name are checked. A
// reference is the name of an installed app or the repository it came from
// (org/repo or host/org/repo), and may be a glob pattern such as "cos*" or
// "sigstore/*". A reference naming no installed app returns an error
// wrapping ErrNotInstalled.
func (dropper *Dropper) CheckUpdates(refs ...string) ([]*UpdateStatus, error) {
	inv, err := inventory.Open()
	if err != nil {
		return nil, fmt.Errorf("opening install inventory: %w", err)
	}

	keys, err := selectInstalled(inv, refs)
	if err != nil {
		return nil, err
	}

	// Several installed apps can come from the same repository, check
	// each repo only once.
	latestCache := map[string]string{}
	errCache := map[string]error{}

	ret := make([]*UpdateStatus, 0, len(keys))
	for _, key := range keys {
		record := inv.Installs[key]
		repoKey := record.Host + "/" + record.Org + "/" + record.Repo

		latest, cached := latestCache[repoKey]
		checkErr := errCache[repoKey]
		if !cached && checkErr == nil {
			latest, checkErr = dropper.latestReleaseVersion(record)
			if checkErr == nil {
				latestCache[repoKey] = latest
			} else {
				errCache[repoKey] = checkErr
			}
		}

		status := &UpdateStatus{
			Record:        record,
			LatestVersion: latest,
			Error:         checkErr,
		}
		if checkErr == nil {
			status.UpdateAvailable = versionIsNewer(record.Version, latest)
		}
		ret = append(ret, status)
	}
	return ret, nil
}

// selectInstalled returns the sorted inventory keys of the apps named by
// refs, or all of them when no refs are given. Each ref is matched as a glob
// against the app name, its org/repo and its host/org/repo. Every ref has to
// name at least one installed app.
func selectInstalled(inv *inventory.Inventory, refs []string) ([]string, error) {
	keys := slices.Sorted(maps.Keys(inv.Installs))
	if len(refs) == 0 {
		return keys, nil
	}

	selected := map[string]bool{}
	for _, ref := range refs {
		pattern := strings.TrimSpace(ref)
		if i := strings.Index(pattern, "://"); i >= 0 {
			pattern = pattern[i+3:]
		}
		pattern = strings.Trim(pattern, "/")
		if _, err := path.Match(pattern, ""); err != nil {
			return nil, fmt.Errorf("invalid app pattern %q: %w", ref, err)
		}

		matched := false
		for _, key := range keys {
			r := inv.Installs[key]
			for _, candidate := range []string{
				r.Name, r.Org + "/" + r.Repo, r.Host + "/" + r.Org + "/" + r.Repo,
			} {
				// The pattern was validated above, Match cannot fail here
				if ok, _ := path.Match(pattern, candidate); ok { //nolint:errcheck // validated
					selected[key] = true
					matched = true
					break
				}
			}
		}
		if !matched {
			return nil, fmt.Errorf("%w: %s", ErrNotInstalled, ref)
		}
	}

	return slices.DeleteFunc(keys, func(key string) bool { return !selected[key] }), nil
}

// latestReleaseVersion returns the tag of the newest stable release in the
// repo an app was installed from, matching how the installer resolves
// "latest" when no version is pinned.
func (dropper *Dropper) latestReleaseVersion(record *inventory.Record) (string, error) {
	release, err := dropper.client.LatestRelease(&github.Repository{
		Host: record.Host,
		Org:  record.Org,
		Repo: record.Repo,
	})
	if err != nil {
		return "", fmt.Errorf("finding latest release: %w", err)
	}
	return release.GetVersion(), nil
}

// updateInstallOptions builds the install options to update an app, honoring
// the choices recorded when it was installed: the artifact kind, the binary
// location, the file picked from an archive and the verification stance.
func updateInstallOptions(record *inventory.Record) []FuncGetOption {
	options := []FuncGetOption{
		WithReinstall(true),
		WithVerifyDownloads(record.Verified),
	}
	switch record.Kind {
	case string(ArtifactBinary):
		options = append(options, WithDownloadType("b"))
		if record.BinPath != "" {
			options = append(options, WithBinDir(filepath.Dir(record.BinPath)))
		}
	case string(ArtifactArchive):
		options = append(options, WithDownloadType("a"), WithArchiveEntry(record.ArchiveEntry))
		if record.BinPath != "" {
			options = append(options, WithBinDir(filepath.Dir(record.BinPath)))
		}
	case string(ArtifactPackage):
		options = append(options, WithDownloadType("p"))
	}
	return options
}

// Update reinstalls an app from the latest release of its repository,
// reusing the choices recorded when it was originally installed.
func (dropper *Dropper) Update(status *UpdateStatus, funcs ...FuncGetOption) error {
	record := status.Record
	spec := &github.Asset{
		Host: record.Host,
		Org:  record.Org,
		Repo: record.Repo,
		Name: record.Name,
	}

	options := updateInstallOptions(record)
	options = append(options, funcs...)

	return dropper.Install(spec, options...)
}

// versionIsNewer compares two release tags, using semver ordering when both
// tags parse and falling back to plain inequality when they don't.
func versionIsNewer(installed, latest string) bool {
	if latest == "" {
		return false
	}
	if installed == "" {
		return true
	}
	iv, ierr := semver.NewVersion(installed)
	lv, lerr := semver.NewVersion(latest)
	if ierr == nil && lerr == nil {
		return lv.GreaterThan(iv)
	}
	return installed != latest
}
