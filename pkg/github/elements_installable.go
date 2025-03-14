// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package github

import (
	"regexp"

	"github.com/carabiner-dev/drop/pkg/system"
)

// Installable abstracts a group of files that are the same app but
// offer variants for different architectures and OSs.
type Installable struct {
	// RepoData
	Host string
	Repo string
	Org  string

	// Release
	Version  string
	Name     string
	Variants []*Asset
}

func assetListToInstallableList(assets []AssetDataProvider) {
	// Find installable clusters
	splitterRegex := regexp.MustCompile(system.MainSplitPattern())
	ret := []AssetDataProvider{}
	installables := map[string]*Installable{}
	for _, asset := range assets {
		// Split on os and arch
		parts := splitterRegex.Split(asset.GetName(), -1)

		// If the split returned only one part, it means we are dealing with
		// an asset without arch/os variants. So we don't treat it as installable
		if len(parts) == 1 {
			ret = append(ret, asset)
			continue
		}

		// Otherwise it is a variant of an installable
		name := trimSeparatorSuffix(parts[0])
		if _, ok := installables[name]; !ok {
			installables[name] = &Installable{
				Host:     asset.GetHost(),
				Repo:     asset.GetRepo(),
				Org:      asset.GetOrg(),
				Version:  asset.GetVersion(),
				Name:     name,
				Variants: []*Asset{},
			}
		}

	}
}

// getArchOsFromFilename reads a filename and looks for the known OS and Arch
// labels in it.
func getArchOsFromFilename(filename string) (string, string) {
	return getArchFromFilename(filename), getOsFromFilename(filename)
}

// getArchFromFilename examines a filename and tries to infer a target
// architecture by looking for the supported labels
func getOsFromFilename(filename string) string {
	for os, aliases := range system.OSAliases {
		if aliases.ToRegex().MatchString(filename) {
			return os
		}
	}
	return ""
}

// getArchFromFilename examines a filename and tries to infer a target
// architecture by looking for the supported labels
func getArchFromFilename(filename string) string {
	for os, aliases := range system.ArchAliases {
		if aliases.ToRegex().MatchString(filename) {
			return os
		}
	}
	return ""
}

// trimSeparatorSuffix trims any separator character found at the end of an installable
// name. This is to trim leftover chars after detecting assets with variants
//
// TODO(puerco): Perhaps this should trim all chars if there is more than one at the end.
func trimSeparatorSuffix(name string) string {
	// installables
	lastChar := name[len(name)-1]
	if _, ok := system.FilenameSeparators[string(lastChar)]; ok {
		return name[0 : len(name)-1]
	}
	return name
}
