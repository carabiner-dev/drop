// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package drop

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/carabiner-dev/drop/pkg/archive"
	"github.com/carabiner-dev/drop/pkg/system"
)

var (
	// ErrNoBinaryInArchive is returned when an archive holds no native
	// executable for the target platform.
	ErrNoBinaryInArchive = errors.New("archive contains no executable for the platform")

	// ErrNoMatchingArchiveEntry is returned when an archive ships executables
	// but none is named after the installable.
	ErrNoMatchingArchiveEntry = errors.New("no executable named after the installable in the archive")
)

// maxExtractedSize caps the size of a binary extracted from an archive,
// guarding against decompression bombs.
const maxExtractedSize = 1 << 30 // 1 GiB

// maxLinkHops bounds symlink resolution inside an archive
const maxLinkHops = 8

// extractDirName is the directory, next to the downloaded archive, where the
// selected file is extracted. The installer removes the whole download
// directory once done.
const extractDirName = "unpacked"

// Data keys of archive events
const (
	dataKeyArchive = "archive"
	dataKeyEntry   = "entry"
	dataKeyFormat  = "format"
)

// sharedLibSuffixes are extensions of shared libraries, native executable
// files that are never the program to install.
var sharedLibSuffixes = []string{".so", ".dylib", ".dll", ".a", ".lib"}

// isSharedLibrary returns true for library filenames, including versioned
// unix ones such as libfoo.so.1.2
func isSharedLibrary(name string) bool {
	lower := strings.ToLower(name)
	if strings.Contains(lower, ".so.") {
		return true
	}
	for _, suffix := range sharedLibSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

// archiveCandidates filters an archive index down to the regular files that
// can be installed as the app binary on the target OS: native executables,
// recognized by their content, that are not shared libraries. On windows the
// file must also carry the .exe extension.
func archiveCandidates(entries []archive.Entry, targetOS string) []archive.Entry {
	candidates := make([]archive.Entry, 0, len(entries))
	for _, e := range entries {
		if !e.IsRegular() || isSharedLibrary(e.Base()) {
			continue
		}
		if !system.IsNativeExecutable(e.Head, targetOS) {
			continue
		}
		if targetOS == system.OSWindows && !strings.EqualFold(path.Ext(e.Path), exeSuffix) {
			continue
		}
		candidates = append(candidates, e)
	}
	return candidates
}

// findEntry returns the entry stored at entryPath or nil.
func findEntry(entries []archive.Entry, entryPath string) *archive.Entry {
	for i := range entries {
		if entries[i].Path == entryPath {
			return &entries[i]
		}
	}
	return nil
}

// resolveLink follows a symlink entry to the regular file it points to
// inside the archive. Absolute targets and targets escaping the archive
// are not followed.
func resolveLink(entries []archive.Entry, link *archive.Entry) *archive.Entry {
	current := link
	for range maxLinkHops {
		if !current.IsSymlink() {
			if current.IsRegular() {
				return current
			}
			return nil
		}
		if current.Linkname == "" || path.IsAbs(current.Linkname) {
			return nil
		}
		target := path.Join(path.Dir(current.Path), current.Linkname)
		if !fs.ValidPath(target) {
			return nil
		}
		if current = findEntry(entries, target); current == nil {
			return nil
		}
	}
	return nil
}

// nameMatches reports if an entry basename is the installable name. On
// windows the .exe extension is implied and case is ignored.
func nameMatches(base, wanted, targetOS string) bool {
	if targetOS == system.OSWindows {
		return strings.EqualFold(base, wanted+exeSuffix) || strings.EqualFold(base, wanted)
	}
	return base == wanted
}

// shallower reports if entry path a sits higher in the archive than b, to
// prefer bin/app over share/examples/app when both match.
func shallower(a, b string) bool {
	da, db := strings.Count(a, "/"), strings.Count(b, "/")
	if da != db {
		return da < db
	}
	if len(a) != len(b) {
		return len(a) < len(b)
	}
	return a < b
}

// matchByName looks among the candidates for the executable named after the
// installable. A symlink with the wanted name resolving to a candidate
// counts as well. The shallowest match wins.
func matchByName(entries, candidates []archive.Entry, wanted, targetOS string) *archive.Entry {
	var best *archive.Entry
	for i := range entries {
		e := &entries[i]
		if !nameMatches(e.Base(), wanted, targetOS) {
			continue
		}
		if e.IsSymlink() {
			if e = resolveLink(entries, e); e == nil {
				continue
			}
		}
		// Only native executables qualify, a text file or a wrapper
		// script named after the app is not the binary.
		if findEntry(candidates, e.Path) == nil {
			continue
		}
		if best == nil || shallower(e.Path, best.Path) {
			best = e
		}
	}
	return best
}

// selectArchiveEntry picks the file to install from the contents of an
// archive: the pinned entry when it is present and installable, otherwise
// the executable named after the installable. When neither applies, the
// error lists the executables found so the caller can choose.
func selectArchiveEntry(entries []archive.Entry, wanted, pinned, targetOS string) (*archive.Entry, error) {
	candidates := archiveCandidates(entries, targetOS)

	if pinned != "" {
		if e := findEntry(candidates, pinned); e != nil {
			return e, nil
		}
	}

	if e := matchByName(entries, candidates, wanted, targetOS); e != nil {
		return e, nil
	}

	if len(candidates) == 0 {
		return nil, ErrNoBinaryInArchive
	}
	names := make([]string, 0, len(candidates))
	for _, c := range candidates {
		names = append(names, c.Path)
	}
	return nil, fmt.Errorf("%w (%q): found %s", ErrNoMatchingArchiveEntry, wanted, strings.Join(names, ", "))
}

// installArchive locates the app binary inside the downloaded archive,
// extracts it next to the archive and installs it like a bare binary.
func (di *defaultImplementation) installArchive(
	opts *GetOptions, info *system.Info, artifact *InstallArtifact, archivePath string,
) error {
	entries, format, err := archive.List(archivePath)
	if err != nil {
		return fmt.Errorf("reading archive: %w", err)
	}

	var entry *archive.Entry
	if format.Container == "" {
		// A bare compressed file holds nothing but the binary, which
		// installs under the installable name.
		candidates := archiveCandidates(entries, opts.OS)
		if len(candidates) == 0 {
			return ErrNoBinaryInArchive
		}
		entry = &candidates[0]
	} else {
		entry, err = selectArchiveEntry(entries, artifact.Name, opts.ArchiveEntry, opts.OS)
		if err != nil {
			return err
		}
		artifact.ArchiveEntry = entry.Path
		// The binary installs under the installable name when the archive
		// exposes it under that name (directly or through a symlink),
		// otherwise it keeps its own.
		if matchByName(entries, []archive.Entry{*entry}, artifact.Name, opts.OS) == nil {
			artifact.InstallName = entry.Base()
		}
	}

	archiveName := filepath.Base(archivePath)
	opts.Listener.HandleEvent(&Event{
		Object: EventObjectArchive, Verb: EventVerbRunning,
		Data: map[string]string{
			dataKeyArchive: archiveName,
			dataKeyEntry:   entry.Path,
			dataKeyFormat:  format.String(),
		},
	})

	dir := filepath.Join(filepath.Dir(archivePath), extractDirName)
	if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("creating extraction directory: %w", err)
	}
	extracted := filepath.Join(dir, entry.Base())
	if err := archive.Extract(archivePath, entry.Path, extracted, maxExtractedSize); err != nil {
		return fmt.Errorf("extracting %s from %s: %w", entry.Path, archiveName, err)
	}

	opts.Listener.HandleEvent(&Event{
		Object: EventObjectArchive, Verb: EventVerbDone,
		Data: map[string]string{
			dataKeyArchive: archiveName,
			dataKeyEntry:   entry.Path,
		},
	})

	return di.installBinary(opts, info, artifact, extracted)
}
