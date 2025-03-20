// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package system

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetTypeFromFile(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		sut    string
		expect string
	}{
		{"zip", "file.zip", "zip"},
		{"tar.gz", "file.tar.gz", "tgz"},
		{"tgz", "file.tgz", "tgz"},
		{"gz", "file.other.gz", "gz"},
		{"bzip-variant-1", "file.other.bz", "bz2"},
		{"bzip-variant-2", "file.other.bz2", "bz2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expect, ArchiveExtensions.GetTypeFromFile(tc.sut))
		})
	}
}

func TestGetTypeExtensionFromFile(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		sut        string
		expectType string
		expectExt  string
	}{
		{"zip", "file.zip", "zip", "zip"},
		{"tar.gz", "file.tar.gz", "tgz", "tar.gz"},
		{"tgz", "file.tgz", "tgz", "tgz"},
		{"gz", "file.other.gz", "gz", "gz"},
		{"bzip-variant-1", "file.other.bz", "bz2", "bz"},
		{"bzip-variant-2", "file.other.bz2", "bz2", "bz2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tp, ext := ArchiveExtensions.GetTypeExtensionFromFile(tc.sut)
			require.Equal(t, tc.expectType, tp)
			require.Equal(t, tc.expectExt, ext)
		})
	}
}
