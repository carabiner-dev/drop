// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package github

type Release struct {
	// Repository
	Host string
	Repo string
	Org  string

	// Release
	Version string
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
