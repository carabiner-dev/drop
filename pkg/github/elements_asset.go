// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package github

import "time"

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
	Size        int64
	Label       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
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
