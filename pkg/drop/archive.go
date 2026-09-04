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

	// ErrBadArchiveEntry is returned when the entry the user asked for is
	// not in the archive or is not an executable for the platform.
	ErrBadArchiveEntry = errors.New("requested archive entry cannot be installed")
)

// ArchiveEntrySelector chooses which executable inside an archive to install
// when none is named after the installable. It receives the archive filename,
// the installable name and the paths of the executables found, and returns
// the chosen path, or ErrInstallAborted when the user declines. The CLI
// injects an interactive implementation when running on a terminal.
type ArchiveEntrySelector func(archiveName, wanted string, candidates []string) (string, error)

// archiveChoice gathers the inputs to pick the file to install from an archive.
type archiveChoice struct {
	// archive is the archive filename, shown when asking the user
	archive string

	// wanted is the installable name
	wanted string

	// pinned is the entry recorded by a previous install of the app, or
	// the one the user asked for
	pinned string

	// required makes a pinned entry that cannot be installed an error
	// instead of falling back to the regular selection
	required bool

	// targetOS is the platform the binary must run on
	targetOS string

	// selector asks which executable to install when none matches wanted
	selector ArchiveEntrySelector
}

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

// entryNames lists the paths of a set of entries for messages
func entryNames(entries []archive.Entry) string {
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Path)
	}
	return strings.Join(names, ", ")
}

// checkRequiredEntry explains why a required entry cannot be installed.
func checkRequiredEntry(entries, candidates []archive.Entry, choice *archiveChoice) error {
	if findEntry(entries, choice.pinned) == nil {
		return fmt.Errorf(
			"%w: %q not found in %s (executables found: %s)",
			ErrBadArchiveEntry, choice.pinned, choice.archive, entryNames(candidates),
		)
	}
	return fmt.Errorf(
		"%w: %q in %s is not a %s executable",
		ErrBadArchiveEntry, choice.pinned, choice.archive, choice.targetOS,
	)
}

// selectArchiveEntry picks the file to install from the contents of an
// archive: the pinned entry when it is present and installable, otherwise
// the executable named after the installable. When neither applies the
// selector is asked to choose among the executables found; without one the
// error lists them. A required pinned entry never falls back.
func selectArchiveEntry(entries []archive.Entry, choice *archiveChoice) (*archive.Entry, error) {
	candidates := archiveCandidates(entries, choice.targetOS)

	if choice.pinned != "" {
		if e := findEntry(candidates, choice.pinned); e != nil {
			return e, nil
		}
		if choice.required {
			return nil, checkRequiredEntry(entries, candidates, choice)
		}
	}

	if e := matchByName(entries, candidates, choice.wanted, choice.targetOS); e != nil {
		return e, nil
	}

	if len(candidates) == 0 {
		return nil, ErrNoBinaryInArchive
	}

	if choice.selector == nil {
		return nil, fmt.Errorf("%w (%q): found %s", ErrNoMatchingArchiveEntry, choice.wanted, entryNames(candidates))
	}

	names := make([]string, 0, len(candidates))
	for _, c := range candidates {
		names = append(names, c.Path)
	}
	chosen, err := choice.selector(choice.archive, choice.wanted, names)
	if err != nil {
		return nil, fmt.Errorf("choosing the file to install from %s: %w", choice.archive, err)
	}
	e := findEntry(candidates, chosen)
	if e == nil {
		return nil, fmt.Errorf("%q is not one of the executables found in %s", chosen, choice.archive)
	}
	return e, nil
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

	archiveName := filepath.Base(archivePath)
	choice := &archiveChoice{
		archive:  archiveName,
		wanted:   artifact.Name,
		pinned:   opts.ArchiveEntry,
		required: opts.ArchiveEntryRequired,
		targetOS: opts.OS,
		selector: opts.EntrySelector,
	}

	var entry *archive.Entry
	if format.Container == "" {
		// A bare compressed file holds nothing but the binary, which
		// installs under the installable name.
		candidates := archiveCandidates(entries, opts.OS)
		if len(candidates) == 0 {
			return ErrNoBinaryInArchive
		}
		if choice.required && choice.pinned != candidates[0].Path {
			return checkRequiredEntry(entries, candidates, choice)
		}
		entry = &candidates[0]
	} else {
		entry, err = selectArchiveEntry(entries, choice)
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
