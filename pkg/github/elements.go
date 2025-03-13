// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package github

type ReleaseDataProvider interface {
	RepoDataProvider
	GetVersion() string
}

type RepoDataProvider interface {
	GetHost() string
	GetRepo() string
	GetOrg() string
}
