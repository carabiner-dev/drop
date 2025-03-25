// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package github

import (
	"time"

	gogithub "github.com/google/go-github/v60/github"
)

func newAssetFromGitHubAsset(src ReleaseDataProvider, asset *gogithub.ReleaseAsset) *Asset {
	arch, os := getArchOsFromFilename(asset.GetName())
	return &Asset{
		Host:        src.GetHost(),
		Org:         src.GetOrg(),
		Repo:        src.GetRepo(),
		Version:     src.GetVersion(),
		Name:        asset.GetName(),
		DownloadURL: asset.GetBrowserDownloadURL(),
		Author:      asset.GetUploader().GetLogin(),
		CreatedAt:   *asset.CreatedAt.GetTime(),
		UpdatedAt:   *asset.UpdatedAt.GetTime(),
		Size:        asset.GetSize(),
		Label:       asset.GetLabel(),
		Os:          os,
		Arch:        arch,
	}
}

// Asset is an abstraction of a file released on GitHub. It captures the basic
// file information but also its type platform and version.
type Asset struct {
	// RepoData
	Host string
	Repo string
	Org  string

	// Release
	Version string

	// Asset
	Name        string
	DownloadURL string
	Author      string
	Size        int
	Label       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Arch        string
	Os          string
}

func (a *Asset) GetHost() string {
	return a.Host
}

func (a *Asset) GetRepo() string {
	return a.Repo
}

func (a *Asset) GetOrg() string {
	return a.Org
}

func (a *Asset) GetVersion() string {
	return a.Version
}

func (a *Asset) GetName() string {
	return a.Name
}

func (a *Asset) GetAuthor() string {
	return a.Author
}

func (a *Asset) GetSize() int {
	return a.Size
}

func (a *Asset) GetCreatedAt() time.Time {
	return a.CreatedAt
}

func (a *Asset) GetUpdatedAt() time.Time {
	return a.UpdatedAt
}

func (a *Asset) GetDownloadURL() string {
	return a.DownloadURL
}

func (a *Asset) GetLabel() string {
	return a.Label
}

func (a *Asset) GetRepoURL() string {
	return buildRepositoryURL(a)
}
