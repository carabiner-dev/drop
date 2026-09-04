// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package system

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	headELF      = []byte{0x7f, 'E', 'L', 'F', 0x02, 0x01, 0x01, 0x00}
	headMachO64  = []byte{0xcf, 0xfa, 0xed, 0xfe, 0x07, 0x00, 0x00, 0x01} // little endian on disk
	headMachO32  = []byte{0xce, 0xfa, 0xed, 0xfe, 0x07, 0x00, 0x00, 0x00}
	headMachOBE  = []byte{0xfe, 0xed, 0xfa, 0xcf, 0x00, 0x00, 0x00, 0x07}
	headFat      = []byte{0xca, 0xfe, 0xba, 0xbe, 0x00, 0x00, 0x00, 0x02}
	headJava     = []byte{0xca, 0xfe, 0xba, 0xbe, 0x00, 0x00, 0x00, 0x41} // class file, major 65
	headPE       = []byte("MZ\x90\x00\x03\x00\x00\x00")
	headScript   = []byte("#!/bin/sh\n")
	headAppleDbl = []byte{0x00, 0x05, 0x16, 0x07, 0x00, 0x02, 0x00, 0x00}
	headGzip     = []byte{0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00}
)

func TestExecutableFormat(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		head   []byte
		expect string
	}{
		{"elf", headELF, ExecutableELF},
		{"elf-short", headELF[:4], ExecutableELF},
		{"macho64", headMachO64, ExecutableMachO},
		{"macho32", headMachO32, ExecutableMachO},
		{"macho-bigendian", headMachOBE, ExecutableMachO},
		{"fat", headFat, ExecutableMachO},
		{"fat-truncated", headFat[:4], ""},
		{"java-class", headJava, ""},
		{"pe", headPE, ExecutablePE},
		{"script", headScript, ""},
		{"appledouble", headAppleDbl, ""},
		{"gzip", headGzip, ""},
		{"text", []byte("hello world"), ""},
		{"empty", []byte{}, ""},
		{"nil", nil, ""},
		{"one-byte", []byte{0x7f}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expect, ExecutableFormat(tc.head))
		})
	}
}

func TestIsNativeExecutable(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		head   []byte
		os     string
		expect bool
	}{
		{"elf-linux", headELF, OSLinux, true},
		{"elf-freebsd", headELF, OSFreeBSD, true},
		{"elf-openbsd", headELF, OSOpenBSD, true},
		{"elf-netbsd", headELF, OSNetBSD, true},
		{"elf-illumos", headELF, OSIllumos, true},
		{"elf-solaris", headELF, OSSolaris, true},
		{"elf-darwin", headELF, OSDarwin, false},
		{"elf-windows", headELF, OSWindows, false},
		{"macho-darwin", headMachO64, OSDarwin, true},
		{"fat-darwin", headFat, OSDarwin, true},
		{"macho-linux", headMachO64, OSLinux, false},
		{"pe-windows", headPE, OSWindows, true},
		{"pe-linux", headPE, OSLinux, false},
		{"pe-darwin", headPE, OSDarwin, false},
		{"elf-any-os", headELF, "", true},
		{"pe-any-os", headPE, "", true},
		{"elf-unknown-os", headELF, "plan9", true},
		{"script-linux", headScript, OSLinux, false},
		{"script-any-os", headScript, "", false},
		{"java-darwin", headJava, OSDarwin, false},
		{"empty-linux", nil, OSLinux, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expect, IsNativeExecutable(tc.head, tc.os))
		})
	}
}
