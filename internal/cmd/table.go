// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/carabiner-dev/termtable"
)

// Column titles shared by the listings.
const (
	colName       = "NAME"
	colVersion    = "VERSION"
	colKind       = "KIND"
	colLocation   = "LOCATION"
	colVerified   = "VERIFIED"
	colUpdated    = "UPDATED"
	colRepository = "REPOSITORY"
)

// listingTable builds the borderless table drop uses for listings: a bold
// header over a rule, a bold first column, single-line cells and every
// column exactly as wide as its widest content.
//
// The table is indented like drop's other output; termtable reserves the
// indent from the terminal width so the table may use the rest of the
// screen. When piped there is no width to honor, so the natural width of
// the content is used.
func listingTable(titles []string, rows [][]string, toTerminal bool) *termtable.Table {
	opts := []termtable.TableOption{
		termtable.WithTableStyle("border: none"),
		termtable.WithIndent(2),
		termtable.WithMaxWidthPercent(100),
	}
	if !toTerminal {
		opts = append(opts, termtable.WithMaxWidth(4096))
	}
	t := termtable.NewTable(opts...)

	// The floor of each column is its content width and a zero weight
	// keeps it from growing.
	for i, title := range titles {
		width := termtable.DisplayWidth(title)
		for _, row := range rows {
			width = max(width, termtable.DisplayWidth(row[i]))
		}
		t.Column(i).SetMin(width).SetWeight(0)
	}

	head := t.AddHeader(termtable.WithRowBorderBottom(termtable.BorderEdgeSolid))
	for _, title := range titles {
		head.AddCell(termtable.WithContent(title), termtable.WithBold(), termtable.WithSingleLine())
	}
	for _, cells := range rows {
		row := t.AddRow()
		for i, content := range cells {
			cellOpts := []termtable.CellOption{termtable.WithContent(content), termtable.WithSingleLine()}
			if i == 0 {
				cellOpts = append(cellOpts, termtable.WithBold())
			}
			row.AddCell(cellOpts...)
		}
	}
	return t
}
