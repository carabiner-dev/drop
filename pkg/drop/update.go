// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package drop

import (
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"

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
func (dropper *Dropper) CheckUpdates() ([]*UpdateStatus, error) {
	inv, err := inventory.Open()
	if err != nil {
		return nil, fmt.Errorf("opening install inventory: %w", err)
	}

	// Several installed apps can come from the same repository, check
	// each repo only once.
	latestCache := map[string]string{}
	errCache := map[string]error{}

	ret := make([]*UpdateStatus, 0, len(inv.Installs))
	for _, key := range slices.Sorted(maps.Keys(inv.Installs)) {
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

// latestReleaseVersion returns the tag of the newest release in the repo an
// app was installed from. The first listed release is used, matching how the
// installer resolves "latest" when no version is pinned.
func (dropper *Dropper) latestReleaseVersion(record *inventory.Record) (string, error) {
	releases, err := dropper.client.ListReleases(&github.Repository{
		Host: record.Host,
		Org:  record.Org,
		Repo: record.Repo,
	})
	if err != nil {
		return "", fmt.Errorf("listing releases: %w", err)
	}
	if len(releases) == 0 {
		return "", errors.New("repository has no releases")
	}
	return releases[0].GetVersion(), nil
}

// updateInstallOptions builds the install options to update an app, honoring
// the choices recorded when it was installed: the artifact kind, the binary
// location and the verification stance.
func updateInstallOptions(record *inventory.Record) []FuncGetOption {
	options := []FuncGetOption{
		WithVerifyDownloads(record.Verified),
	}
	switch record.Kind {
	case string(ArtifactBinary):
		options = append(options, WithDownloadType("b"))
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
