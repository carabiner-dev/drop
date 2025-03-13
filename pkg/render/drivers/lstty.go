// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package drivers

import (
	"fmt"
	"io"
	"time"

	"github.com/carabiner-dev/drop/pkg/github"
	"github.com/rodaine/table"
)

func NewLsTTY() *LsTTY {
	return &LsTTY{
		Options: Options{
			Long: true,
		},
	}
}

type LsTTY struct {
	Options Options
}

type Options struct {
	Long bool
}

func (ls *LsTTY) RenderReleaseAssets(w io.Writer, release github.ReleaseDataProvider, assets []github.AssetDataProvider) error {
	if ls.Options.Long {
		tbl := table.New("perms", "owner", "org", "size", "month", "day", "hour", "name")
		tbl.WithHeaderFormatter(func(format string, vals ...interface{}) string {
			return ""
		})
		tbl.WithWriter(w)

		for _, a := range assets {
			m := a.GetUpdatedAt().Month().String()
			d := fmt.Sprintf("%d", a.GetUpdatedAt().Day())
			h := fmt.Sprintf("%2d:%2d", a.GetUpdatedAt().Local().Hour(), a.GetUpdatedAt().Local().Minute())
			if a.GetUpdatedAt().Year() != time.Now().Year() {
				h = fmt.Sprintf("%d", a.GetUpdatedAt().Year())
			}
			tbl.AddRow("assset", a.GetAuthor(), release.GetOrg(), a.GetSize(), m, d, h, a.GetName())
		}

		tbl.Print()
	}
	return nil
}

func (ls *LsTTY) RenderRepoReleases(w io.Writer, repo github.RepoDataProvider, releases []github.ReleaseDataProvider) error {
	return nil
}
