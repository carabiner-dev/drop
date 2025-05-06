// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package system

import (
	"bufio"
	"cmp"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
)

var (
	regexCache         = sync.Map{}
	FilenameSeparators = map[string]string{
		"-": "-", "_": "_", ".": ".",
	}
)

// GetArch returns the "official" (according to this library) arch label
// from a string. If it does not match one of the known aliases it returns an
// empty string
func GetArch(label string) string {
	for arch, aliases := range ArchAliases {
		if slices.Contains(aliases, label) {
			return arch
		}
	}
	return ""
}

// GetOS returns the "official" (according to this library) Os label
// from a string. If it does not match one of the known aliases it returns an
// empty string
func GetOS(label string) string {
	for os, aliases := range OSAliases {
		if slices.Contains(aliases, label) {
			return os
		}
	}
	return ""
}

// MainSplitPattern dynamically builds a regex pattern with the know OS and arch
// patterns to split and parse filenames to deduct platform, kind and other data.
func MainSplitPattern() string {
	all := []string{}
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

// LabelList is a list of os or arch labels.
type LabelList []string

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

	rany, ok := regexCache.Load(pattern)
	if ok {
		if r, ok2 := rany.(*regexp.Regexp); ok2 {
			return r
		}
	}

	// If not store it in the cache and return
	r := regexp.MustCompile(pattern)
	regexCache.Store(pattern, r)
	return r
}

// GetPreferredPackage returns the preferred package format for a system family
func GetPreferredPackage(family string) string {
	switch family {
	case OSFamilyAlpine, OSFamilyWolfi:
		return PackageApk
	case OSFamilyDebian, OSFamilyUbuntu:
		return PackageDeb
	case OSFamilyAlma, OSFamilyArch, OSFamilyFedora, OSFamilyRocky, OSFamilyRHEL:
		return PackageRPM
	case OSFamilyMacOS:
		return PackageDmg
	case OSFamilyWindows:
		return PackageMSI
	default:
		return ""
	}
}

// parseOSRelease returns the
func parseOSReleaseForFamily(r io.Reader) string {
	if r == nil {
		return ""
	}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		k, v, ok := strings.Cut(scanner.Text(), "=")
		if !ok {
			continue
		}

		if k != "ID" {
			continue
		}
		v = strings.TrimSpace(v)
		if strings.HasPrefix(v, `"`) && strings.HasSuffix(v, `"`) {
			v = strings.TrimPrefix(v, `"`)
			v = strings.TrimSuffix(v, `"`)
		}

		switch v {
		case "alpine":
			return OSFamilyAlpine
		case "almalinux":
			return OSFamilyAlma
		case "arch":
			return OSFamilyArch
		case "fedora":
			return OSFamilyFedora
		case "debian":
			return OSFamilyDebian
		case "distroless":
			return OSFamilyDistroless
		case "rocky":
			return OSFamilyRocky
		case "rhel":
			return OSFamilyRHEL
		case "ubuntu":
			return OSFamilyUbuntu
		case "wolfi":
			return OSFamilyWolfi
		default:
			fmt.Println("A A A A A " + v)
			return ""
		}
	}
	return ""
}

// GetSystemOSFamily returns the constant representing the local system
func GetSystemOSFamily() string {
	// We don't really have families on these two
	switch runtime.GOOS {
	case "windows":
		return OSFamilyWindows
	case "darwin":
		return OSFamilyMacOS
	}

	// If not win or max, parse the OS release file
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return ""
	}
	defer f.Close() //nolint:errcheck
	return parseOSReleaseForFamily(f)
}
