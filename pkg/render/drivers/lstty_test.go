// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package drivers

import (
	"strings"
	"testing"

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
