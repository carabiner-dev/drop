// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package system

import (
	"bytes"
	"debug/elf"
	"debug/macho"
	"encoding/binary"
)

// Executable formats recognized by their magic numbers
const (
	ExecutableELF   = "elf"
	ExecutableMachO = "macho"
	ExecutablePE    = "pe"
)

// ExecutableHeadSize is the number of leading bytes IsNativeExecutable needs
// to recognize every supported format.
const ExecutableHeadSize = 8

// maxFatArches bounds the number of architectures in a universal (fat) Mach-O
// header. Java class files share the fat magic number but their version
// field, sitting in the same position, is always larger.
const maxFatArches = 30

// peMagic is the DOS stub signature that starts every Windows executable.
var peMagic = []byte("MZ")

// ExecutableFormat inspects the leading bytes of a file and returns the
// native executable format they announce, or an empty string when the data
// does not start with a known executable magic number.
func ExecutableFormat(head []byte) string {
	switch {
	case isELF(head):
		return ExecutableELF
	case isMachO(head):
		return ExecutableMachO
	case isPE(head):
		return ExecutablePE
	default:
		return ""
	}
}

// IsNativeExecutable returns true when the leading bytes of a file look like
// a native executable for the specified operating system: ELF for linux and
// the unix-likes, Mach-O for darwin and PE for windows. An empty OS matches
// any of the known formats. Detection is by content only, the executable
// permission bits are not taken into account.
func IsNativeExecutable(head []byte, osName string) bool {
	format := ExecutableFormat(head)
	if format == "" {
		return false
	}
	switch osName {
	case OSWindows:
		return format == ExecutablePE
	case OSDarwin:
		return format == ExecutableMachO
	case OSLinux, OSFreeBSD, OSNetBSD, OSOpenBSD, OSIllumos, OSSolaris:
		return format == ExecutableELF
	default:
		return true
	}
}

func isELF(head []byte) bool {
	return bytes.HasPrefix(head, []byte(elf.ELFMAG))
}

// isMachO checks for the thin Mach-O magic numbers in both byte orders and
// for the universal binary header.
func isMachO(head []byte) bool {
	if len(head) < 4 {
		return false
	}
	be := binary.BigEndian.Uint32(head)
	le := binary.LittleEndian.Uint32(head)
	switch {
	case be == macho.Magic32, le == macho.Magic32, be == macho.Magic64, le == macho.Magic64:
		return true
	case be == macho.MagicFat:
		// Fat headers are always big-endian, the arch count follows the magic.
		return len(head) >= 8 && binary.BigEndian.Uint32(head[4:8]) < maxFatArches
	default:
		return false
	}
}

func isPE(head []byte) bool {
	return bytes.HasPrefix(head, peMagic)
}
