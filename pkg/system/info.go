// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package system

import (
	"cmp"
	"regexp"
	"runtime"
	"slices"
	"strings"
)

func GetInfo() (*Info, error) {
	return &Info{
		Os:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}, nil
}

var FilenameSeparators = map[string]string{
	"-": "-", "_": "_", ".": ".",
}

const (
	OSWindows = "windows"
	OSLinux   = "linux"
	OSDarwin  = "darwin"
	OSFreeBSD = "freebsd"

	// Base arches
	ArchX8664   = "x86_64"
	Arch386     = "386"
	ArchArm     = "arm"
	ArchArm64   = "arm64"
	ArchRiscV64 = "riscv64"

	// Not supported
	ArchS390X   = "s390x"   // IBM Z
	ArchPPC64LE = "ppc64le" // IBM Power (redhat naming)
	ArchPPC64EL = "ppc64el" // IBM Power (debian naming)
	ArchPPC64   = "ppc64"

	// Aliases
	ArchArmHF   = "armhf"
	ArchArmV7   = "armv7"
	ArchArmV7HL = "armv7hl"
	ArchAarch64 = "aarch64"
	ArchAMD64   = "amd64"
	ArchX86     = "x86"
	ArchI686    = "i686"
	ArchI386    = "i386"
)

func MainSplitPattern() string {
	var all = []string{}
	for _, c := range ArchAliases {
		all = append(all, c...)
	}

	for _, c := range OSAliases {
		all = append(all, c...)
	}

	// Sort by string length
	// slices.Sort(all)
	slices.SortFunc(all,
		func(a, b string) int {
			if len(a) == len(b) {
				return cmp.Compare(a, b)
			}
			return cmp.Compare(len(a), len(b)) * -1
		})

	return "(?i)(" + strings.Join(all, "|") + ")"

}

type LabelList []string

var regexCache = map[string]*regexp.Regexp{}

func (ll *LabelList) ToRegex() *regexp.Regexp {
	list := slices.Clone(*ll)
	chrs := []string{}
	for c := range FilenameSeparators {
		chrs = append(chrs, c)
	}
	slices.Sort(chrs)
	// Terms need to be sorted by length so that the
	// longer strings match first
	slices.SortFunc(list,
		func(a, b string) int {
			if len(a) == len(b) {
				return cmp.Compare(a, b)
			}
			return cmp.Compare(len(a), len(b)) * -1
		},
	)

	// We need to take into account the filename separators as we need
	// to avoid a match of "arm" on "arm64.exe
	sepChars := `[` + strings.Join(chrs, "") + `]`
	for i := range list {
		item := list[i]
		list[i] = item + sepChars
		// ... but also the arch at end of the string
		list[i] += "|" + item + "$"
	}

	// Build the whole pattern
	pattern := "(?i)(" + strings.Join(list, "|") + ")"
	if _, ok := regexCache[pattern]; !ok {
		regexCache[pattern] = regexp.MustCompile(pattern)
	}
	return regexCache[pattern]
}

var OSAliases = map[string]LabelList{
	OSLinux:   {OSLinux},
	OSWindows: {OSWindows},
	OSDarwin:  {OSDarwin},
	OSFreeBSD: {OSFreeBSD},
}

var ArchAliases = map[string]LabelList{
	ArchX8664:   {ArchX8664, ArchAMD64},
	ArchArm64:   {ArchArm64, ArchAarch64},
	ArchArm:     {ArchArm, ArchArmHF, ArchArmV7, ArchArmV7HL},
	Arch386:     {Arch386, ArchI686, ArchX86, ArchI386},
	ArchRiscV64: {ArchRiscV64},
	ArchS390X:   {ArchS390X},
	ArchPPC64LE: {ArchPPC64LE, ArchPPC64EL, ArchPPC64},
}

const (
	PackageRPM = "rpm"
	PackageDeb = "deb"
	PackageApk = "apk"
	PackageDmg = "dmg"

	ArchiveZip = "zip"
	ArchiveTar = "tar"
	ArchiveBz  = "bz"
	ArchiveBz2 = "bz2"
	ArchiveGz  = "gz"
	ArchiveXz  = "xz"
	ArchiveRar = "rar"
	ArchiveL7  = "l7"
	ArchiveTgz = "tgz"
)

var PackageTypes = []string{PackageRPM, PackageDeb, PackageApk, PackageDmg}
var ArchiveTypes = []string{ArchiveZip, ArchiveTar, ArchiveBz, ArchiveBz2, ArchiveGz, ArchiveXz, ArchiveRar, ArchiveL7, ArchiveTgz}

type Info struct {
	Os   string
	Arch string
}
