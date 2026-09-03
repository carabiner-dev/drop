// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

// Package archive lists and extracts single files from release archives.
//
// It handles zip files, tarballs (plain or compressed with gzip, bzip2, xz
// or zstd) and bare compressed files, detecting the format from the data
// and falling back to the filename extension. Extraction is deliberately
// limited to one regular file at a time, written to a path chosen by the
// caller and capped in size, so the installer never trusts paths or sizes
// stored in an archive.
package archive

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path"
	"strings"

	"github.com/carabiner-dev/drop/pkg/system"
)

var (
	ErrUnknownFormat  = errors.New("unrecognized archive format")
	ErrEntryNotFound  = errors.New("entry not found in archive")
	ErrNotRegularFile = errors.New("archive entry is not a regular file")
	ErrTooLarge       = errors.New("archive entry exceeds the maximum extraction size")
)

// headSize is the number of leading bytes captured from regular files, enough
// to recognize the native executable formats.
const headSize = system.ExecutableHeadSize

// maxLinknameSize caps the symlink targets read from zip entries
const maxLinknameSize = 4096

// Entry describes a file stored in an archive.
type Entry struct {
	// Path is the slash separated, cleaned path of the entry inside the
	// archive. It is always relative and never escapes the archive root.
	Path string

	// Mode carries the entry type (regular file, directory or symlink)
	// and the permission bits recorded in the archive.
	Mode fs.FileMode

	// Linkname is the target of symbolic link entries, as stored.
	Linkname string

	// Head holds the leading bytes of regular files for content sniffing.
	// It is shorter than headSize when the file is.
	Head []byte
}

// Base returns the last element of the entry path.
func (e *Entry) Base() string {
	return path.Base(e.Path)
}

// IsRegular returns true if the entry is a regular file.
func (e *Entry) IsRegular() bool {
	return e.Mode.IsRegular()
}

// IsDir returns true if the entry is a directory.
func (e *Entry) IsDir() bool {
	return e.Mode.IsDir()
}

// IsSymlink returns true if the entry is a symbolic link.
func (e *Entry) IsSymlink() bool {
	return e.Mode&fs.ModeSymlink != 0
}

// cleanName normalizes an entry name to a slash separated relative path and
// reports whether it is safe to use. Absolute names and names escaping the
// archive root through parent references are rejected.
func cleanName(name string) (string, bool) {
	name = path.Clean(strings.ReplaceAll(name, `\`, "/"))
	if name == "." || !fs.ValidPath(name) {
		return "", false
	}
	return name, true
}

// List returns the entries stored in the archive at path along with the
// detected format. Only regular files, directories and symlinks are
// listed; hard links, devices and metadata headers are skipped, as are
// entries with unsafe names. When the same name appears more than once,
// the first occurrence wins.
func List(archivePath string) ([]Entry, Format, error) {
	s, err := open(archivePath)
	if err != nil {
		return nil, Format{}, err
	}
	defer s.Close() //nolint:errcheck

	var entries []Entry
	switch {
	case s.zip != nil:
		entries, err = listZip(s.zip)
	case s.format.Container == ContainerTar:
		entries, err = listTar(s.r)
	default:
		entries, err = listBare(s)
	}
	if err != nil {
		return nil, s.format, err
	}
	return entries, s.format, nil
}

// readHead reads the leading bytes of an entry, returning fewer when the
// entry is shorter.
func readHead(r io.Reader) ([]byte, error) {
	buf := make([]byte, headSize)
	n, err := io.ReadFull(r, buf)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, err
	}
	return buf[:n], nil
}

func listTar(r io.Reader) ([]Entry, error) {
	tr := tar.NewReader(r)
	entries := []Entry{}
	seen := map[string]struct{}{}
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return entries, nil
		}
		if err != nil {
			return nil, fmt.Errorf("reading tar entry: %w", err)
		}

		perm := fs.FileMode(hdr.Mode).Perm() //nolint:gosec // G115: Perm masks the value to 9 bits
		entry := Entry{Mode: perm}
		switch hdr.Typeflag {
		case tar.TypeReg:
		case tar.TypeDir:
			entry.Mode |= fs.ModeDir
		case tar.TypeSymlink:
			entry.Mode |= fs.ModeSymlink
			entry.Linkname = hdr.Linkname
		default:
			// Hard links, devices, fifos and global pax headers
			continue
		}

		name, ok := cleanName(hdr.Name)
		if !ok {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		entry.Path = name

		if hdr.Typeflag == tar.TypeReg {
			head, err := readHead(tr)
			if err != nil {
				return nil, fmt.Errorf("reading %s: %w", name, err)
			}
			entry.Head = head
		}
		entries = append(entries, entry)
	}
}

// readZipEntry opens a zip entry and reads up to limit bytes from it.
func readZipEntry(f *zip.File, limit int) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close() //nolint:errcheck
	buf := make([]byte, limit)
	n, err := io.ReadFull(rc, buf)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, err
	}
	return buf[:n], nil
}

func listZip(zr *zip.ReadCloser) ([]Entry, error) {
	entries := make([]Entry, 0, len(zr.File))
	seen := map[string]struct{}{}
	for _, f := range zr.File {
		name, ok := cleanName(f.Name)
		if !ok {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}

		entry := Entry{Path: name, Mode: f.Mode()}
		switch {
		case entry.IsDir() || strings.HasSuffix(f.Name, "/"):
			entry.Mode |= fs.ModeDir
		case entry.IsSymlink():
			target, err := readZipEntry(f, maxLinknameSize)
			if err != nil {
				return nil, fmt.Errorf("reading link target of %s: %w", name, err)
			}
			entry.Linkname = string(target)
		case entry.IsRegular():
			head, err := readZipEntry(f, headSize)
			if err != nil {
				return nil, fmt.Errorf("reading %s: %w", name, err)
			}
			entry.Head = head
		default:
			continue
		}
		seen[name] = struct{}{}
		entries = append(entries, entry)
	}
	return entries, nil
}

// bareEntryName names the single file inside a bare compressed file: the
// archive name without its compression extension.
func bareEntryName(name string) string {
	switch t, ext := system.ArchiveExtensions.GetTypeExtensionFromFile(name); t {
	case system.ArchiveGz, system.ArchiveXz, system.ArchiveBz2, system.ArchiveZst:
		return strings.TrimSuffix(name, "."+ext)
	default:
		return name
	}
}

func listBare(s *stream) ([]Entry, error) {
	head, err := s.r.Peek(headSize)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, fmt.Errorf("reading compressed data: %w", err)
	}
	return []Entry{{
		Path: bareEntryName(s.name),
		Mode: 0o644,
		Head: bytes.Clone(head),
	}}, nil
}

// Extract writes the regular file stored at entryPath in the archive to dst.
// The output is truncated if it exists and created with mode 0600. At most
// maxSize bytes are written: larger entries fail with ErrTooLarge and the
// partial output is removed. A maxSize of zero or less disables the cap.
func Extract(archivePath, entryPath, dst string, maxSize int64) error {
	s, err := open(archivePath)
	if err != nil {
		return err
	}
	defer s.Close() //nolint:errcheck

	var r io.Reader
	switch {
	case s.zip != nil:
		rc, err := openZipEntry(s.zip, entryPath)
		if err != nil {
			return err
		}
		defer rc.Close() //nolint:errcheck
		r = rc
	case s.format.Container == ContainerTar:
		r, err = seekTarEntry(s.r, entryPath)
		if err != nil {
			return err
		}
	default:
		if entryPath != bareEntryName(s.name) {
			return fmt.Errorf("%w: %s", ErrEntryNotFound, entryPath)
		}
		r = s.r
	}
	return writeCapped(r, dst, maxSize)
}

// seekTarEntry advances the tar stream to the regular file named entryPath.
func seekTarEntry(r io.Reader, entryPath string) (io.Reader, error) {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("%w: %s", ErrEntryNotFound, entryPath)
		}
		if err != nil {
			return nil, fmt.Errorf("reading tar entry: %w", err)
		}
		name, ok := cleanName(hdr.Name)
		if !ok || name != entryPath {
			continue
		}
		if hdr.Typeflag != tar.TypeReg {
			return nil, fmt.Errorf("%w: %s", ErrNotRegularFile, entryPath)
		}
		return tr, nil
	}
}

// openZipEntry opens the regular file named entryPath in a zip archive.
func openZipEntry(zr *zip.ReadCloser, entryPath string) (io.ReadCloser, error) {
	for _, f := range zr.File {
		name, ok := cleanName(f.Name)
		if !ok || name != entryPath {
			continue
		}
		if !f.Mode().IsRegular() || strings.HasSuffix(f.Name, "/") {
			return nil, fmt.Errorf("%w: %s", ErrNotRegularFile, entryPath)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("opening zip entry: %w", err)
		}
		return rc, nil
	}
	return nil, fmt.Errorf("%w: %s", ErrEntryNotFound, entryPath)
}

// writeCapped copies r to a new file at dst, failing if it grows past maxSize.
func writeCapped(r io.Reader, dst string, maxSize int64) error {
	if maxSize <= 0 {
		maxSize = math.MaxInt64 - 1
	}
	f, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) //nolint:gosec // dst is chosen by the caller, never by the archive
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}

	// Ask for one byte more than allowed: if the copy is satisfied in full,
	// the entry is too large.
	_, err = io.CopyN(f, r, maxSize+1)
	switch {
	case err == nil:
		_ = f.Close()      //nolint:errcheck
		_ = os.Remove(dst) //nolint:errcheck
		return fmt.Errorf("%w (%d bytes)", ErrTooLarge, maxSize)
	case !errors.Is(err, io.EOF):
		_ = f.Close()      //nolint:errcheck
		_ = os.Remove(dst) //nolint:errcheck
		return fmt.Errorf("extracting entry: %w", err)
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(dst) //nolint:errcheck
		return fmt.Errorf("closing output file: %w", err)
	}
	return nil
}
