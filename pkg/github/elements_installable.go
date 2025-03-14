// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package github

import (
	"cmp"
	"regexp"
	"slices"
	"strings"
	"time"

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
	Version string

	Name     string
	Variants []*Asset

	// Asset
	DownloadURL string
	Author      string
	Size        int
	Label       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Arch        string
	Os          string
}

func (i *Installable) LocalVariant() *Asset {
	info, err := system.GetInfo()
	if err != nil {
		return nil
	}
	for _, v := range i.Variants {
		if v.Os == info.Os && v.Arch == info.Arch {
			return v
		}
	}
	return nil
}

func (i *Installable) GetOsVariants() []string {
	ret := []string{}
	for _, v := range i.Variants {
		if v.Os == "" {
			continue
		}
		if !slices.Contains(ret, v.Os) {
			ret = append(ret, v.Os)
		}
	}
	return ret
}

func (i *Installable) GetArchVariants() []string {
	ret := []string{}
	for _, v := range i.Variants {
		if v.Arch == "" {
			continue
		}
		if !slices.Contains(ret, v.Arch) {
			ret = append(ret, v.Arch)
		}
	}
	return ret
}

// assetListToInstallableList takes a list of assets and organizes them into
// consolidated installables or plain asssets.
func assetListToInstallableList(assets []AssetDataProvider) []AssetDataProvider {
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

		arch, os := getArchOsFromFilename(asset.GetName())
		installables[name].Variants = append(installables[name].Variants,
			&Asset{
				Host:        asset.GetHost(),
				Repo:        asset.GetRepo(),
				Org:         asset.GetOrg(),
				Version:     asset.GetVersion(),
				Name:        asset.GetName(),
				DownloadURL: asset.GetDownloadURL(),
				Author:      asset.GetAuthor(),
				Size:        asset.GetSize(),
				Label:       asset.GetLabel(),
				CreatedAt:   asset.GetCreatedAt(),
				UpdatedAt:   asset.GetUpdatedAt(),
				Arch:        arch,
				Os:          os,
			},
		)
	}

	for _, i := range installables {
		ret = append(ret, i)
	}

	slices.SortFunc(ret, func(a, b AssetDataProvider) int {
		return cmp.Compare(a.GetName(), a.GetName())
	})

	return ret
}

// getArchOsFromFilename reads a filename and looks for the known OS and Arch
// labels in it.
func getArchOsFromFilename(filename string) (arch, os string) {
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

	// If it's a package then we know
	if strings.HasSuffix(filename, ".rpm") || strings.HasSuffix(filename, ".deb") || strings.HasSuffix(filename, ".apk") {
		return system.OSLinux
	}

	if strings.HasSuffix(filename, ".exe") {
		return system.OSWindows
	}

	if strings.HasSuffix(filename, ".dmg") {
		return system.OSDarwin
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

func (i *Installable) GetHost() string {
	return i.Host
}

func (i *Installable) GetRepo() string {
	return i.Repo
}

func (i *Installable) GetOrg() string {
	return i.Org
}

func (i *Installable) GetVersion() string {
	return i.Version
}

func (i *Installable) GetName() string {
	return i.Name
}

func (i *Installable) GetAuthor() string {
	v := i.LocalVariant()
	if v != nil {
		return v.Author
	}

	return i.Author
}

func (i *Installable) GetSize() int {
	v := i.LocalVariant()
	if v != nil {
		return v.Size
	}
	return i.Size
}

func (i *Installable) GetCreatedAt() time.Time {
	v := i.LocalVariant()
	if v != nil {
		return v.CreatedAt
	}
	return i.CreatedAt
}

func (i *Installable) GetUpdatedAt() time.Time {
	v := i.LocalVariant()
	if v != nil {
		return v.UpdatedAt
	}

	return i.UpdatedAt
}

func (i *Installable) GetDownloadURL() string {
	v := i.LocalVariant()
	if v != nil {
		return v.DownloadURL
	}

	return i.DownloadURL
}

func (i *Installable) GetLabel() string {
	return i.Label
}
