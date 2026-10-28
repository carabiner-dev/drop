// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package drivers

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestColumnTablePadsLastRow(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		numCols int
		data    []string
	}{
		{name: "partial-last-row", numCols: 3, data: []string{"one", "two", "three", "four"}},
		{name: "single-cell", numCols: 3, data: []string{"lonely"}},
		{name: "full-rows", numCols: 3, data: []string{"a1", "a2", "a3"}},
		{name: "two-columns", numCols: 2, data: []string{"b1", "b2", "b3"}},
		{name: "empty", numCols: 3, data: []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var b strings.Builder
			columnTable(&b, tc.numCols, tc.data)
			out := b.String()
			require.NotContains(t, out, "MISSING")
			for _, cell := range tc.data {
				require.Contains(t, out, cell)
			}
		})
	}
}

func TestColumnTableLayout(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	columnTable(&b, 3, []string{"one", "two", "three", "four", "five"})
	lines := strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
	require.Len(t, lines, 2, "five names in three columns take two rows")
	require.Regexp(t, `^\s*one\s+two\s+three\s*$`, lines[0])
	require.Regexp(t, `^\s*four\s+five\s*$`, lines[1])
}

func TestLongRow(t *testing.T) {
	t.Parallel()
	now := time.Now()
	thisYear := time.Date(now.Year(), time.March, 5, 9, 7, 0, 0, time.Local)
	row := longRow("perms", "me", "org", 42, thisYear, "thing")
	require.Equal(t, []string{"perms", "me", "org", "42", "Mar", "5", "09:07", "thing"}, row)

	old := time.Date(now.Year()-3, time.December, 25, 23, 59, 0, 0, time.Local)
	row = longRow("perms", "me", "org", 0, old, "thing")
	require.Equal(t, strconv.Itoa(now.Year()-3), row[6], "items from other years show the year instead of the hour")
}

func TestLongTableFitsAndCounts(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	rows := [][]string{
		longRow("📄➖➖➖➖➖➖➖", "bot", "org", 10, time.Now(), "a.txt"),
		longRow("💾🐧🍏🪟📦➖➖➖", "bot", "org", 200, time.Now(), "installable"),
	}
	require.NoError(t, longTable(&b, rows))
	lines := strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
	require.Len(t, lines, 3)
	require.Equal(t, "total 2", lines[0])
	require.Contains(t, lines[1], "a.txt")
	require.Contains(t, lines[2], "installable")
}
