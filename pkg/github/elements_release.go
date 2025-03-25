// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package github

import (
	"time"

	gogithub "github.com/google/go-github/v60/github"
)

func newReleaseFromGitHubRelease(
	repo RepoDataProvider,
	release *gogithub.RepositoryRelease,
) *Release {
	return &Release{
		Host:       repo.GetHost(),
		Repo:       repo.GetRepo(),
		Org:        repo.GetOrg(),
		Version:    release.GetTagName(),
		ID:         release.GetID(),
		PreRelease: release.GetPrerelease(),
		CreatedAt:  *release.CreatedAt.GetTime(),
		Author:     release.GetAuthor().GetLogin(),
	}
}

// Release captures the information of a GitHub release.
type Release struct {
	// Repository
	Host string
	Repo string
	Org  string

	// Release
	Version    string
	ID         int64
	PreRelease bool
	CreatedAt  time.Time
	Author     string
}

func (r *Release) GetHost() string {
	return r.Host
}

func (r *Release) GetRepo() string {
	return r.Repo
}

func (r *Release) GetOrg() string {
	return r.Org
}

func (r *Release) GetVersion() string {
	return r.Version
}

func (r *Release) GetCreatedAt() time.Time {
	return r.CreatedAt
}

func (r *Release) GetAuthor() string {
	return r.Author
}

func (r *Release) GetRepoURL() string {
	return buildRepositoryURL(r)
}
