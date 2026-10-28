// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package drivers

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"time"

	"github.com/carabiner-dev/termtable"
	"github.com/mattn/go-isatty"

	"github.com/carabiner-dev/drop/pkg/github"
	"github.com/carabiner-dev/drop/pkg/system"
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

// shortColumns is the number of columns the short listing lays names out in.
const shortColumns = 3

// lsTable builds the headerless, borderless table the ls listings share.
// Every column is exactly as wide as its content, except the last one,
// which takes whatever space is left: it is where long names are trimmed
// when the terminal is too narrow for them.
//
// On a terminal the table may use the whole width; when writing anywhere
// else there is no width to honor, so the natural width of the content
// is used.
func lsTable(w io.Writer, rows [][]string) *termtable.Table {
	opts := []termtable.TableOption{
		termtable.WithTableStyle("border: none"),
		termtable.WithTablePadding(termtable.Padding{Right: 1}),
		termtable.WithMinWidth(1),
	}
	if isTerminal(w) {
		opts = append(opts, termtable.WithMaxWidthPercent(100))
	} else {
		opts = append(opts, termtable.WithMaxWidth(4096))
	}
	t := termtable.NewTable(opts...)
	if len(rows) == 0 {
		return t
	}

	numCols := len(rows[0])
	for i := range numCols - 1 {
		width := 0
		for _, row := range rows {
			width = max(width, termtable.DisplayWidth(row[i]))
		}
		t.Column(i).SetMin(width).SetWeight(0)
	}
	for _, cells := range rows {
		row := t.AddRow()
		for _, content := range cells {
			row.AddCell(termtable.WithContent(content), termtable.WithSingleLine())
		}
	}
	return t
}

// isTerminal reports whether w is an interactive terminal.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && isatty.IsTerminal(f.Fd())
}

// columnTable lays data out in numCols columns, row by row, like a short
// ls listing. The last row is padded so every row has numCols cells.
func columnTable(w io.Writer, numCols int, data []string) {
	rows := [][]string{}
	for start := 0; start < len(data); start += numCols {
		cols := make([]string, numCols)
		copy(cols, data[start:min(start+numCols, len(data))])
		rows = append(rows, cols)
	}
	_, _ = lsTable(w, rows).WriteTo(w) //nolint:errcheck
}

// longTable prints the "total" line and the rows of a long listing.
func longTable(w io.Writer, rows [][]string) error {
	if _, err := fmt.Fprintf(w, "total %d\n", len(rows)); err != nil {
		return err
	}
	t := lsTable(w, rows)
	if len(rows) > 0 {
		t.Column(3).SetAlign(termtable.AlignRight) // size
		t.Column(5).SetAlign(termtable.AlignRight) // day
	}
	_, err := t.WriteTo(w)
	return err
}

// longRow builds one row of a long listing: the permission-like
// indicators, who published the item and where, its size, when it was
// published and its name. The time shows the hour for items from the
// current year and the year otherwise, like ls -l.
func longRow(perms, owner, org string, size int, when time.Time, name string) []string {
	when = when.Local()
	hour := when.Format("15:04")
	if when.Year() != time.Now().Year() {
		hour = strconv.Itoa(when.Year())
	}
	return []string{perms, owner, org, strconv.Itoa(size), when.Format("Jan"), strconv.Itoa(when.Day()), hour, name}
}

func permString(item github.AssetDataProvider) string {
	str := []rune("T➖➖➖➖➖➖➖")
	switch artifact := item.(type) {
	case *github.Installable:
		str[0] = '💾'
		oss := artifact.GetOsVariants()
		if slices.Contains(oss, system.OSLinux) {
			str[1] = '🐧'
		}
		if slices.Contains(oss, system.OSDarwin) {
			str[2] = '🍏'
		}
		if slices.Contains(oss, system.OSWindows) {
			str[3] = '🪟'
		}
		if len(artifact.GetPackageTypes()) > 0 {
			str[4] = '📦'
		}
		if len(artifact.GetArchiveTypes()) > 0 {
			str[5] = '🎁'
		}
	case *github.Asset:
		str[0] = '📄'
		if artifact.Os == system.OSLinux {
			str[1] = '🐧'
		}
		if artifact.Os == system.OSDarwin {
			str[2] = '🍏'
		}
		if artifact.Os == system.OSWindows {
			str[3] = '🪟'
		}
		if system.IsPackage(artifact.GetName()) {
			str[4] = '📦'
		}
		if system.IsArchive(artifact.GetName()) {
			str[5] = '🎁'
		}
	}

	return string(str)
}

// renderAssets lists release assets or installables, long or short.
func (ls *LsTTY) renderAssets(w io.Writer, release github.ReleaseDataProvider, assets []github.AssetDataProvider) error {
	if !ls.Options.Long {
		data := make([]string, 0, len(assets))
		for _, a := range assets {
			data = append(data, a.GetName())
		}
		columnTable(w, shortColumns, data)
		return nil
	}
	rows := make([][]string, 0, len(assets))
	for _, a := range assets {
		rows = append(rows, longRow(permString(a), a.GetAuthor(), release.GetOrg(), a.GetSize(), a.GetUpdatedAt(), a.GetName()))
	}
	return longTable(w, rows)
}

func (ls *LsTTY) RenderReleaseInstallables(w io.Writer, release github.ReleaseDataProvider, assets []github.AssetDataProvider) error {
	return ls.renderAssets(w, release, assets)
}

func (ls *LsTTY) RenderReleaseAssets(w io.Writer, release github.ReleaseDataProvider, assets []github.AssetDataProvider) error {
	return ls.renderAssets(w, release, assets)
}

func (ls *LsTTY) RenderRepoReleases(w io.Writer, _ github.RepoDataProvider, releases []github.ReleaseDataProvider) error {
	if !ls.Options.Long {
		data := make([]string, 0, len(releases))
		for _, r := range releases {
			data = append(data, r.GetVersion())
		}
		columnTable(w, shortColumns, data)
		return nil
	}
	rows := make([][]string, 0, len(releases))
	for _, r := range releases {
		rows = append(rows, longRow("release", r.GetAuthor(), r.GetOrg(), 0, r.GetCreatedAt(), r.GetVersion()))
	}
	return longTable(w, rows)
}
