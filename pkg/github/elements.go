// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package github

import (
	"fmt"
	"time"
)

type ReleaseDataProvider interface {
	RepoDataProvider
	GetVersion() string
	GetCreatedAt() time.Time
	GetAuthor() string
}

type RepoDataProvider interface {
	GetHost() string
	GetRepo() string
	GetOrg() string
	GetRepoURL() string
}

type AssetDataProvider interface {
	ReleaseDataProvider

	GetName() string
	GetAuthor() string
	GetSize() int
	GetCreatedAt() time.Time
	GetUpdatedAt() time.Time
	GetDownloadURL() string
	GetLabel() string
}

func buildRepositoryURL(provider RepoDataProvider) string {
	return fmt.Sprintf("https://%s/%s/%s", provider.GetHost(), provider.GetOrg(), provider.GetRepo())
}
