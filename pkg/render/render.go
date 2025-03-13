// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package render

import (
	"io"

	"github.com/carabiner-dev/drop/pkg/github"
	"github.com/carabiner-dev/drop/pkg/render/drivers"
)

func New() *Engine {
	return &Engine{
		driver: drivers.NewLsTTY(),
	}
}

type Engine struct {
	driver Driver
}

func (e *Engine) RenderReleaseAssets(w io.Writer, release github.ReleaseDataProvider, assets []github.AssetDataProvider) error {
	return e.driver.RenderReleaseAssets(w, release, assets)
}
func (e *Engine) RenderRepoReleases(w io.Writer, repo github.RepoDataProvider, releases []github.ReleaseDataProvider) error {
	return e.driver.RenderRepoReleases(w, repo, releases)
}

type Driver interface {
	RenderReleaseAssets(io.Writer, github.ReleaseDataProvider, []github.AssetDataProvider) error
	RenderRepoReleases(io.Writer, github.RepoDataProvider, []github.ReleaseDataProvider) error
}
