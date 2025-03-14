// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package drivers

import (
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/carabiner-dev/drop/pkg/github"
	"github.com/carabiner-dev/drop/pkg/system"
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

func columnTable(w io.Writer, numCols int, data []string) {
	headers := make([]interface{}, numCols)
	tbl := table.New(headers...)
	tbl.WithHeaderFormatter(func(format string, vals ...interface{}) string {
		return ""
	})
	tbl.WithWriter(w)

	rows := [][]string{}
	cols := []string{}

	for _, cell := range data {
		if len(cols) == numCols {
			rows = append(rows, cols)
			cols = []string{}
		}
		cols = append(cols, cell)
	}
	if len(cols) != 0 {
		rows = append(rows, cols)
	}
	tbl.SetRows(rows).Print()
}

func permString(item github.AssetDataProvider) string {
	str := []rune("T➖➖➖➖➖➖➖")
	if inst, ok := item.(*github.Installable); ok {
		str[0] = '💾'
		oss := inst.GetOsVariants()
		if slices.Contains(oss, system.OSLinux) {
			str[1] = '🐧'
		}
		if slices.Contains(oss, system.OSDarwin) {
			str[2] = '🍏'
		}
		if slices.Contains(oss, system.OSDarwin) {
			str[3] = '🪟'
		}
		if len(inst.GetPackageTypes()) > 0 {
			str[4] = '📦'
		}
		if len(inst.GetArchiveTypes()) > 0 {
			str[5] = '🎁'
		}
	} else if asst, ok := item.(*github.Asset); ok {
		str[0] = '📄'
		if asst.Os == system.OSLinux {
			str[1] = '🐧'
		} else {
			str[1] = '➖'
		}
		if asst.Os == system.OSDarwin {
			str[2] = '🍏'
		} else {
			str[2] = '➖'
		}
		if asst.Os == system.OSWindows {
			str[3] = '🪟'
		} else {
			str[3] = '➖'
		}
	}

	return string(str)
}

func (ls *LsTTY) RenderReleaseInstallables(w io.Writer, release github.ReleaseDataProvider, assets []github.AssetDataProvider) error {
	if ls.Options.Long {
		tbl := table.New("perms", "owner", "org", "size", "month", "day", "hour", "name")
		tbl.WithHeaderFormatter(func(format string, vals ...any) string {
			return fmt.Sprintf("total %d\n", len(assets))
		})
		tbl.WithWriter(w)

		for _, a := range assets {
			m := a.GetUpdatedAt().Month().String()[0:3]
			d := fmt.Sprintf("%d", a.GetUpdatedAt().Day())
			h := fmt.Sprintf("%2d:%2d", a.GetUpdatedAt().Local().Hour(), a.GetUpdatedAt().Local().Minute())
			if a.GetUpdatedAt().Year() != time.Now().Year() {
				h = fmt.Sprintf("%d", a.GetUpdatedAt().Year())
			}
			tbl.AddRow(permString(a), a.GetAuthor(), release.GetOrg(), a.GetSize(), m, d, h, a.GetName())
		}

		tbl.Print()
	} else {
		data := []string{}
		for _, a := range assets {
			data = append(data, a.GetName())
		}
		columnTable(w, 3, data)
	}
	return nil
}

func (ls *LsTTY) RenderReleaseAssets(w io.Writer, release github.ReleaseDataProvider, assets []github.AssetDataProvider) error {
	if ls.Options.Long {
		tbl := table.New("perms", "owner", "org", "size", "month", "day", "hour", "name")
		tbl.WithHeaderFormatter(func(format string, vals ...interface{}) string {
			return fmt.Sprintf("total %d\n", len(assets))
		})
		tbl.WithWriter(w)

		for _, a := range assets {
			m := a.GetUpdatedAt().Month().String()[0:3]
			d := fmt.Sprintf("%d", a.GetUpdatedAt().Day())
			h := fmt.Sprintf("%2d:%2d", a.GetUpdatedAt().Local().Hour(), a.GetUpdatedAt().Local().Minute())
			if a.GetUpdatedAt().Year() != time.Now().Year() {
				h = fmt.Sprintf("%d", a.GetUpdatedAt().Year())
			}
			tbl.AddRow(permString(a), a.GetAuthor(), release.GetOrg(), a.GetSize(), m, d, h, a.GetName())
		}

		tbl.Print()
	} else {
		data := []string{}
		for _, a := range assets {
			data = append(data, a.GetName())
		}
		columnTable(w, 3, data)
	}
	return nil
}

func (ls *LsTTY) RenderRepoReleases(w io.Writer, repo github.RepoDataProvider, releases []github.ReleaseDataProvider) error {
	if ls.Options.Long {
		tbl := table.New("perms", "owner", "org", "size", "month", "day", "hour", "name")
		tbl.WithHeaderFormatter(func(format string, vals ...interface{}) string {
			return fmt.Sprintf("total %d\n", len(releases))
		})
		tbl.WithWriter(w)

		for _, r := range releases {
			m := r.GetCreatedAt().Month().String()[0:3]
			d := fmt.Sprintf("%d", r.GetCreatedAt().Local().Day())
			h := fmt.Sprintf("%2d:%2d", r.GetCreatedAt().Local().Hour(), r.GetCreatedAt().Local().Minute())
			if r.GetCreatedAt().Year() != time.Now().Year() {
				h = fmt.Sprintf("%d", r.GetCreatedAt().Local().Year())
			}
			tbl.AddRow("release", r.GetAuthor(), r.GetOrg(), "0", m, d, h, r.GetVersion())
		}

		tbl.Print()
	} else {
		data := []string{}
		for _, r := range releases {
			data = append(data, r.GetVersion())
		}
		columnTable(w, 3, data)
	}
	return nil
}
