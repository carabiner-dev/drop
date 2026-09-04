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
		{"zip", "file.zip", ArchiveZip},
		{"tar.gz", "file.tar.gz", ArchiveTgz},
		{"tgz", "file.tgz", ArchiveTgz},
		{"gz", "file.other.gz", ArchiveGz},
		{"bzip-variant-1", "file.other.bz", ArchiveBz2},
		{"bzip-variant-2", "file.other.bz2", ArchiveBz2},
		{"xz", "file.xz", ArchiveXz},
		{"tar.xz", "file.tar.xz", ArchiveTxz},
		{"txz", "file.txz", ArchiveTxz},
		{"tar.bz2", "file.tar.bz2", ArchiveTbz},
		{"tbz2", "file.tbz2", ArchiveTbz},
		{"tbz", "file.tbz", ArchiveTbz},
		{"zst", "file.zst", ArchiveZst},
		{"tar.zst", "file.tar.zst", ArchiveTzst},
		{"tzst", "file.tzst", ArchiveTzst},
		{"7z", "file.7z", Archive7z},
		{"not-an-archive", "file-linux-amd64", ""},
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
		{"zip", "file.zip", ArchiveZip, ArchiveZip},
		{"tar.gz", "file.tar.gz", ArchiveTgz, extTarGz},
		{"tgz", "file.tgz", ArchiveTgz, ArchiveTgz},
		{"gz", "file.other.gz", ArchiveGz, ArchiveGz},
		{"bzip-variant-1", "file.other.bz", ArchiveBz2, "bz"},
		{"bzip-variant-2", "file.other.bz2", ArchiveBz2, ArchiveBz2},
		{"tar.xz", "file.tar.xz", ArchiveTxz, extTarXz},
		{"tar.bz2", "file.tar.bz2", ArchiveTbz, extTarBz2},
		{"tbz2", "file.tbz2", ArchiveTbz, extTbz2},
		{"tar.zst", "file.tar.zst", ArchiveTzst, extTarZst},
		{"zst", "file.other.zst", ArchiveZst, ArchiveZst},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tp, ext := ArchiveExtensions.GetTypeExtensionFromFile(tc.sut)
			require.Equal(t, tc.expectType, tp)
			require.Equal(t, tc.expectExt, ext)
		})
	}
}
