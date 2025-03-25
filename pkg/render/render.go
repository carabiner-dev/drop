// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package render

import (
	"errors"
	"io"

	"github.com/carabiner-dev/drop/pkg/github"
	"github.com/carabiner-dev/drop/pkg/render/drivers"
)

type optFn func(*Engine) error

func WithDriver(driver Driver) optFn {
	return func(e *Engine) error {
		if driver == nil {
			return errors.New("no render driver specified")
		}
		e.driver = driver
		return nil
	}
}

func New(funcs ...optFn) (*Engine, error) {
	e := &Engine{
		driver: drivers.NewLsTTY(),
	}
	for _, fn := range funcs {
		if err := fn(e); err != nil {
			return nil, err
		}
	}
	return e, nil
}

type Engine struct {
	driver Driver
}

type Driver interface {
	RenderReleaseAssets(io.Writer, github.ReleaseDataProvider, []github.AssetDataProvider) error
	RenderRepoReleases(io.Writer, github.RepoDataProvider, []github.ReleaseDataProvider) error
	RenderReleaseInstallables(io.Writer, github.ReleaseDataProvider, []github.AssetDataProvider) error
}

func (e *Engine) RenderReleaseInstallables(w io.Writer, release github.ReleaseDataProvider, assets []github.AssetDataProvider) error {
	return e.driver.RenderReleaseInstallables(w, release, assets)
}

func (e *Engine) RenderReleaseAssets(w io.Writer, release github.ReleaseDataProvider, assets []github.AssetDataProvider) error {
	return e.driver.RenderReleaseAssets(w, release, assets)
}

func (e *Engine) RenderRepoReleases(w io.Writer, repo github.RepoDataProvider, releases []github.ReleaseDataProvider) error {
	return e.driver.RenderRepoReleases(w, repo, releases)
}
