// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package github

import "time"

type ReleaseDataProvider interface {
	RepoDataProvider
	GetVersion() string
}

type RepoDataProvider interface {
	GetHost() string
	GetRepo() string
	GetOrg() string
}

type AssetDataProvider interface {
	ReleaseDataProvider

	GetName() string
	GetAuthor() string
	GetSize() int
	GetCreatedAt() time.Time
	GetUpdatedAt() time.Time
}
