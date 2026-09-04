// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package archive

import (
	"archive/zip"
	"bufio"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"

	"github.com/carabiner-dev/drop/pkg/system"
)

// Container formats
const (
	ContainerTar = "tar"
	ContainerZip = "zip"
)

// Compression formats
const (
	CompressionGzip  = "gzip"
	CompressionBzip2 = "bzip2"
	CompressionXz    = "xz"
	CompressionZstd  = "zstd"
)

// Format describes how an archive is put together: the container holding
// the entries and the compression applied to it. A bare compressed file (a
// gzipped binary, for example) has a compression but no container.
type Format struct {
	Container   string
	Compression string
}

// String returns a label for the format such as "tar+gzip", "zip" or "xz".
func (f Format) String() string {
	switch {
	case f.Container != "" && f.Compression != "":
		return f.Container + "+" + f.Compression
	case f.Container != "":
		return f.Container
	default:
		return f.Compression
	}
}

// Magic numbers announcing the supported formats
var (
	magicGzip  = []byte{0x1f, 0x8b}
	magicBzip2 = []byte("BZh")
	magicXz    = []byte{0xfd, '7', 'z', 'X', 'Z', 0x00}
	magicZstd  = []byte{0x28, 0xb5, 0x2f, 0xfd}
	magicTar   = []byte("ustar")
	magicZips  = [][]byte{
		[]byte("PK\x03\x04"), // local file header
		[]byte("PK\x05\x06"), // empty archive
		[]byte("PK\x07\x08"), // spanned archive
	}
)

const (
	// tarMagicOffset is the position of the ustar magic in a tar header
	tarMagicOffset = 257

	// tarHeaderSize is the size of a tar header block. It is also the amount
	// of data sniffed from a file (and from a decompressed stream) to detect
	// the format, as the tar magic sits near the end of the first block.
	tarHeaderSize = 512
)

// detectCompression returns the compression format announced by the leading
// bytes of a file or an empty string if the data is not compressed.
func detectCompression(head []byte) string {
	switch {
	case bytes.HasPrefix(head, magicGzip):
		return CompressionGzip
	case bytes.HasPrefix(head, magicBzip2):
		return CompressionBzip2
	case bytes.HasPrefix(head, magicXz):
		return CompressionXz
	case bytes.HasPrefix(head, magicZstd):
		return CompressionZstd
	default:
		return ""
	}
}

func isZipHead(head []byte) bool {
	for _, magic := range magicZips {
		if bytes.HasPrefix(head, magic) {
			return true
		}
	}
	return false
}

func isTarHead(head []byte) bool {
	end := tarMagicOffset + len(magicTar)
	return len(head) >= end && bytes.Equal(head[tarMagicOffset:end], magicTar)
}

// formatFromExtension maps the archive type deduced from a filename to a
// format. It is the fallback for files whose data does not announce it, such
// as tarballs without the ustar magic or self extracting zip files.
func formatFromExtension(name string) Format {
	switch system.ArchiveExtensions.GetTypeFromFile(name) {
	case system.ArchiveZip:
		return Format{Container: ContainerZip}
	case system.ArchiveTar:
		return Format{Container: ContainerTar}
	case system.ArchiveTgz:
		return Format{Container: ContainerTar, Compression: CompressionGzip}
	case system.ArchiveTxz:
		return Format{Container: ContainerTar, Compression: CompressionXz}
	case system.ArchiveTbz:
		return Format{Container: ContainerTar, Compression: CompressionBzip2}
	case system.ArchiveTzst:
		return Format{Container: ContainerTar, Compression: CompressionZstd}
	case system.ArchiveGz:
		return Format{Compression: CompressionGzip}
	case system.ArchiveXz:
		return Format{Compression: CompressionXz}
	case system.ArchiveBz2:
		return Format{Compression: CompressionBzip2}
	case system.ArchiveZst:
		return Format{Compression: CompressionZstd}
	default:
		return Format{}
	}
}

// newDecompressor wraps r with the decoder for the compression format.
func newDecompressor(r io.Reader, compression string) (io.ReadCloser, error) {
	switch compression {
	case CompressionGzip:
		gr, err := gzip.NewReader(r)
		if err != nil {
			return nil, fmt.Errorf("opening gzip stream: %w", err)
		}
		return gr, nil
	case CompressionBzip2:
		return io.NopCloser(bzip2.NewReader(r)), nil
	case CompressionXz:
		xr, err := xz.NewReader(r)
		if err != nil {
			return nil, fmt.Errorf("opening xz stream: %w", err)
		}
		return io.NopCloser(xr), nil
	case CompressionZstd:
		// A single decoding goroutine keeps the decoder from spawning
		// workers for every archive we inspect.
		dec, err := zstd.NewReader(r, zstd.WithDecoderConcurrency(1))
		if err != nil {
			return nil, fmt.Errorf("opening zstd stream: %w", err)
		}
		return dec.IOReadCloser(), nil
	default:
		return nil, fmt.Errorf("unsupported compression %q", compression)
	}
}

// stream is an open archive ready to be read.
type stream struct {
	format Format

	// name is the archive filename
	name string

	// r holds the decompressed data of tar containers and bare files
	r *bufio.Reader

	// zip is set instead of r for zip containers
	zip *zip.ReadCloser

	// closers are released in reverse order when the stream is closed
	closers []io.Closer
}

// Close releases the archive and its decoders.
func (s *stream) Close() error {
	var errs []error
	if s.zip != nil {
		errs = append(errs, s.zip.Close())
	}
	for i := len(s.closers) - 1; i >= 0; i-- {
		errs = append(errs, s.closers[i].Close())
	}
	return errors.Join(errs...)
}

// open opens the archive at path, detects its format and returns a stream
// positioned at the start of the container data.
func open(path string) (*stream, error) {
	f, err := os.Open(path) //nolint:gosec // the archive path is chosen by the installer
	if err != nil {
		return nil, fmt.Errorf("opening archive: %w", err)
	}

	head := make([]byte, tarHeaderSize)
	n, err := io.ReadFull(f, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		_ = f.Close() //nolint:errcheck
		return nil, fmt.Errorf("reading archive header: %w", err)
	}
	head = head[:n]

	name := filepath.Base(path)
	extFormat := formatFromExtension(name)
	compression := detectCompression(head)

	if compression == "" {
		container := extFormat.Container
		switch {
		case isZipHead(head):
			container = ContainerZip
		case isTarHead(head):
			container = ContainerTar
		}
		switch container {
		case ContainerZip:
			_ = f.Close() //nolint:errcheck
			zr, err := zip.OpenReader(path)
			if err != nil {
				return nil, fmt.Errorf("opening zip archive: %w", err)
			}
			return &stream{format: Format{Container: ContainerZip}, name: name, zip: zr}, nil
		case ContainerTar:
			if _, err := f.Seek(0, io.SeekStart); err != nil {
				_ = f.Close() //nolint:errcheck
				return nil, fmt.Errorf("rewinding archive: %w", err)
			}
			return &stream{
				format: Format{Container: ContainerTar}, name: name,
				r: bufio.NewReaderSize(f, tarHeaderSize), closers: []io.Closer{f},
			}, nil
		default:
			_ = f.Close() //nolint:errcheck
			return nil, fmt.Errorf("%w: %s", ErrUnknownFormat, name)
		}
	}

	// The data is compressed. Decode it and peek at the first block to see
	// if there is a tarball inside, otherwise it is a bare compressed file.
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		_ = f.Close() //nolint:errcheck
		return nil, fmt.Errorf("rewinding archive: %w", err)
	}
	dec, err := newDecompressor(f, compression)
	if err != nil {
		_ = f.Close() //nolint:errcheck
		return nil, err
	}
	s := &stream{
		format: Format{Compression: compression}, name: name,
		r: bufio.NewReaderSize(dec, tarHeaderSize), closers: []io.Closer{f, dec},
	}

	peek, err := s.r.Peek(tarHeaderSize)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		_ = s.Close() //nolint:errcheck
		return nil, fmt.Errorf("reading archive data: %w", err)
	}
	if isTarHead(peek) || extFormat.Container == ContainerTar {
		s.format.Container = ContainerTar
	}
	return s, nil
}
