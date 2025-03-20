// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package system

var (
	PackageTypes = []string{PackageRPM, PackageDeb, PackageApk, PackageDmg, PackageMSI}
	ArchiveTypes = []string{ArchiveZip, ArchiveTar, ArchiveBz, ArchiveBz2, ArchiveGz, ArchiveXz, ArchiveRar, ArchiveL7, ArchiveTgz}
)

// OS alias maps
var OSAliases = map[string]LabelList{
	OSLinux:   {OSLinux},
	OSWindows: {OSWindows},
	OSDarwin:  {OSDarwin, OSMacOS, OSX},
	OSFreeBSD: {OSFreeBSD},
	OSNetBSD:  {OSNetBSD},
	OSIllumos: {OSIllumos},
	OSSolaris: {OSSolaris},
	OSOpenBSD: {OSOpenBSD},
}

// Arch alias maps
var ArchAliases = map[string]LabelList{
	ArchX8664:   {ArchX8664, ArchAMD64, Arch64Bit, ArchX64},
	ArchArm64:   {ArchArm64, ArchAarch64},
	ArchArm:     {ArchArm, ArchArmHF, ArchArmV7, ArchArmV7HL},
	Arch386:     {Arch386, ArchI686, ArchX86, ArchI386, Arch32Bit},
	ArchRiscV64: {ArchRiscV64},
	ArchS390X:   {ArchS390X},
	ArchPPC64LE: {ArchPPC64LE, ArchPPC64EL, ArchPPC64},
}

// Platform constants
const (
	OSWindows = "windows"
	OSLinux   = "linux"
	OSDarwin  = "darwin"
	OSFreeBSD = "freebsd"
	OSNetBSD  = "netbsd"
	OSIllumos = "illumos"
	OSSolaris = "solaris"
	OSOpenBSD = "openbsd"

	// OS Aliases
	OSMacOS = "macos"
	OSX     = "osx"

	// Base arches
	ArchX8664     = "x86_64"
	Arch386       = "386"
	ArchArm       = "arm"
	ArchArm64     = "arm64"
	ArchRiscV64   = "riscv64"
	ArchUniversal = "universal"

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
	Arch32Bit   = "32bit"
	Arch64Bit   = "64bit"
	ArchX64     = "x64"
)

// Recognized package types
const (
	PackageRPM = "rpm"
	PackageDeb = "deb"
	PackageApk = "apk"
	PackageDmg = "dmg"
	PackageMSI = "msi"
	PackageWhl = "whl" // Python wheel

	ArchiveZip = "zip"
	ArchiveTar = "tar"
	ArchiveBz  = "bz"
	ArchiveBz2 = "bz2"
	ArchiveGz  = "gz"
	ArchiveXz  = "xz"
	ArchiveRar = "rar"
	ArchiveL7  = "l7"
	ArchiveTgz = "tgz"
	Archive7z  = "7z"
)
