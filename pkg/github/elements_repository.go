// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package github

// Repository is the basic construct that exposes information of a repository.
type Repository struct {
	Host string
	Repo string
	Org  string
}

func (r *Repository) GetHost() string {
	return r.Host
}

func (r *Repository) GetRepo() string {
	return r.Repo
}

func (r *Repository) GetOrg() string {
	return r.Org
}

func (r *Repository) GetRepoURL() string {
	return buildRepositoryURL(r)
}
