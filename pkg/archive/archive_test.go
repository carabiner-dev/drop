// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package archive

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"
	"github.com/ulikunitz/xz"
)

// The standard layout every container fixture is built from
const (
	binContent = "\x7fELF\x02\x01\x01\x00fake binary payload\n"
	docContent = "# app\n"
	linkTarget = "../app"

	entryDir  = "app-1.0"
	entryBin  = "app-1.0/app"
	entryDoc  = "app-1.0/README.md"
	entryLink = "app-1.0/bin/app"

	// bareEntry is the entry name of the bare compressed fixtures
	bareEntry = "app"
)

// The standard library cannot write bzip2 data, so the bzip2 fixtures are
// checked in. Both were generated from the standard layout above (written
// with archive/tar: app-1.0/ 0755, app-1.0/app 0755 binContent,
// app-1.0/README.md 0644 docContent, app-1.0/bin/app -> ../app) with:
//
//	bzip2 -9 app.tar && bzip2 -9 app
const (
	fixtureTarBz2 = "QlpoOTFBWSZTWQMwXikAAMDfkfuQSAP/hCcGslB3L94ghEAgAACIMAC4oiT1DETQ09TIMmIwGgmmEwGAAAAAGg0AaAaASiRqMnqAMgNABkAAyepaDf2LiwSLEjwtCjIgBJTFJ4BoRtVsAF6HGxBwRglxQRbueEBL12sEvH5S0M2aQNTjysKoTCgiMCyDAETdPRO0NoKCDjFsCMIHKybXtrzMFCpygeEJCi0YRDgIkByWinNK6TAlQCY3KkRpr821AI5IoqvdNv6QUUUlTgLFZBHg8Ho+pslsE/i7kinChIAZgvFI"
	fixtureBz2    = "QlpoOTFBWSZTWawhlCQAAA3VgHAQQAADBDct0CCgADFGjIGjTI0KABoZHokHRrklYiBhTQJ6WgtptBvi7kinChIVhDKEgA=="
)

type testEntry struct {
	name     string
	typeflag byte
	mode     int64
	content  string
	linkname string
}

func standardLayout() []testEntry {
	return []testEntry{
		{name: entryDir + "/", typeflag: tar.TypeDir, mode: 0o755},
		{name: entryBin, typeflag: tar.TypeReg, mode: 0o755, content: binContent},
		{name: entryDoc, typeflag: tar.TypeReg, mode: 0o644, content: docContent},
		{name: entryLink, typeflag: tar.TypeSymlink, mode: 0o777, linkname: linkTarget},
	}
}

func buildTar(t *testing.T, entries []testEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, e := range entries {
		hdr := &tar.Header{Name: e.name, Typeflag: e.typeflag, Mode: e.mode, Linkname: e.linkname}
		switch e.typeflag {
		case tar.TypeReg:
			hdr.Size = int64(len(e.content))
		case tar.TypeXGlobalHeader:
			hdr.Format = tar.FormatPAX
			hdr.PAXRecords = map[string]string{"comment": "0123456789abcdef"}
		}
		require.NoError(t, tw.WriteHeader(hdr))
		if e.typeflag == tar.TypeReg {
			_, err := tw.Write([]byte(e.content))
			require.NoError(t, err)
		}
	}
	require.NoError(t, tw.Close())
	return buf.Bytes()
}

// buildZip writes the entries to a zip. With unixMode set, entries carry
// unix permission bits (and types); otherwise they look like a zip produced
// on Windows, with only the msdos attributes.
func buildZip(t *testing.T, entries []testEntry, unixMode bool) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		hdr := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		if unixMode {
			mode := fs.FileMode(e.mode) //nolint:gosec // G115: test modes are small
			switch e.typeflag {
			case tar.TypeDir:
				mode |= fs.ModeDir
			case tar.TypeSymlink:
				mode |= fs.ModeSymlink
			}
			hdr.SetMode(mode)
		}
		w, err := zw.CreateHeader(hdr)
		require.NoError(t, err)
		content := e.content
		if e.typeflag == tar.TypeSymlink {
			content = e.linkname
		}
		if content != "" {
			_, err := w.Write([]byte(content))
			require.NoError(t, err)
		}
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func gzipData(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, err := w.Write(data)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return buf.Bytes()
}

func xzData(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, err := xz.NewWriter(&buf)
	require.NoError(t, err)
	_, err = w.Write(data)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return buf.Bytes()
}

func zstdData(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, err := zstd.NewWriter(&buf, zstd.WithEncoderConcurrency(1))
	require.NoError(t, err)
	_, err = w.Write(data)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return buf.Bytes()
}

func decodeFixture(t *testing.T, b64 string) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(b64)
	require.NoError(t, err)
	return data
}

func writeFixture(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(p, data, 0o600))
	return p
}

// standardFixtures returns the standard layout packed in every supported
// container format, keyed by filename.
func standardFixtures(t *testing.T) map[string][]byte {
	t.Helper()
	tarData := buildTar(t, standardLayout())
	return map[string][]byte{
		"app.tar":     tarData,
		"app.tar.gz":  gzipData(t, tarData),
		"app.tgz":     gzipData(t, tarData),
		"app.tar.xz":  xzData(t, tarData),
		"app.tar.zst": zstdData(t, tarData),
		"app.tar.bz2": decodeFixture(t, fixtureTarBz2),
		"app.zip":     buildZip(t, standardLayout(), true),
	}
}

func requireStandardEntries(t *testing.T, entries []Entry) {
	t.Helper()
	require.Len(t, entries, 4)

	require.Equal(t, entryDir, entries[0].Path)
	require.True(t, entries[0].IsDir())
	require.False(t, entries[0].IsRegular())

	bin := entries[1]
	require.Equal(t, entryBin, bin.Path)
	require.Equal(t, bareEntry, bin.Base())
	require.True(t, bin.IsRegular())
	require.False(t, bin.IsSymlink())
	require.Equal(t, []byte(binContent[:headSize]), bin.Head)
	require.Equal(t, fs.FileMode(0o755), bin.Mode.Perm())

	doc := entries[2]
	require.Equal(t, entryDoc, doc.Path)
	require.True(t, doc.IsRegular())
	require.Equal(t, []byte(docContent), doc.Head, "short files yield a short head")

	link := entries[3]
	require.Equal(t, entryLink, link.Path)
	require.True(t, link.IsSymlink())
	require.False(t, link.IsRegular())
	require.Equal(t, linkTarget, link.Linkname)
}

func TestListFormats(t *testing.T) {
	t.Parallel()
	for name, data := range standardFixtures(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			entries, format, err := List(writeFixture(t, name, data))
			require.NoError(t, err)
			require.Equal(t, formatFromExtension(name), format)
			requireStandardEntries(t, entries)
		})
	}
}

func TestListDetectsByContent(t *testing.T) {
	t.Parallel()
	tarData := buildTar(t, standardLayout())
	for _, tc := range []struct {
		name   string
		data   []byte
		expect Format
	}{
		{"tar-gz-as-bin", gzipData(t, tarData), Format{Container: ContainerTar, Compression: CompressionGzip}},
		{"tar-zst-as-bin", zstdData(t, tarData), Format{Container: ContainerTar, Compression: CompressionZstd}},
		{"plain-tar-as-bin", tarData, Format{Container: ContainerTar}},
		{"zip-as-bin", buildZip(t, standardLayout(), true), Format{Container: ContainerZip}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			entries, format, err := List(writeFixture(t, "app.bin", tc.data))
			require.NoError(t, err)
			require.Equal(t, tc.expect, format)
			requireStandardEntries(t, entries)
		})
	}
}

func TestListBare(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		data        []byte
		expect      Format
		expectEntry string
	}{
		{"app.gz", gzipData(t, []byte(binContent)), Format{Compression: CompressionGzip}, bareEntry},
		{"app-linux-amd64.xz", xzData(t, []byte(binContent)), Format{Compression: CompressionXz}, "app-linux-amd64"},
		{"app.zst", zstdData(t, []byte(binContent)), Format{Compression: CompressionZstd}, bareEntry},
		{"app.bz2", decodeFixture(t, fixtureBz2), Format{Compression: CompressionBzip2}, bareEntry},
		{"blob", gzipData(t, []byte(binContent)), Format{Compression: CompressionGzip}, "blob"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			entries, format, err := List(writeFixture(t, tc.name, tc.data))
			require.NoError(t, err)
			require.Equal(t, tc.expect, format)
			require.Len(t, entries, 1)
			require.Equal(t, tc.expectEntry, entries[0].Path)
			require.True(t, entries[0].IsRegular())
			require.Equal(t, []byte(binContent[:headSize]), entries[0].Head)
		})
	}
}

func TestListSkipsUnsafeTarEntries(t *testing.T) {
	t.Parallel()
	data := buildTar(t, []testEntry{
		{name: "pax_global_header", typeflag: tar.TypeXGlobalHeader},
		{name: "./" + entryBin, typeflag: tar.TypeReg, mode: 0o755, content: binContent},
		{name: "../evil", typeflag: tar.TypeReg, mode: 0o755, content: binContent},
		{name: "/abs/evil", typeflag: tar.TypeReg, mode: 0o755, content: binContent},
		{name: "app-1.0/../../evil", typeflag: tar.TypeReg, mode: 0o755, content: binContent},
		{name: "app-1.0/hard", typeflag: tar.TypeLink, mode: 0o755, linkname: entryBin},
		{name: "app-1.0/fifo", typeflag: tar.TypeFifo, mode: 0o644},
		{name: entryBin, typeflag: tar.TypeReg, mode: 0o644, content: "second copy"},
		{name: "app-1.0/empty", typeflag: tar.TypeReg, mode: 0o644},
		{name: `win\bin\app.exe`, typeflag: tar.TypeReg, mode: 0o644, content: "MZ"},
	})

	entries, _, err := List(writeFixture(t, "app.tar", data))
	require.NoError(t, err)

	paths := make([]string, 0, len(entries))
	for _, e := range entries {
		paths = append(paths, e.Path)
	}
	require.Equal(t, []string{entryBin, "app-1.0/empty", "win/bin/app.exe"}, paths)
	require.Equal(t, []byte(binContent[:headSize]), entries[0].Head, "the first duplicate wins")
	require.Empty(t, entries[1].Head)
	require.Equal(t, []byte("MZ"), entries[2].Head)
}

func TestListZipWindowsCreator(t *testing.T) {
	t.Parallel()
	data := buildZip(t, []testEntry{
		{name: "app/", typeflag: tar.TypeDir},
		{name: "app/app.exe", typeflag: tar.TypeReg, content: "MZ\x90\x00\x03\x00\x00\x00\x04"},
		{name: "app/x.dll", typeflag: tar.TypeReg, content: "MZ\x90\x00\x03\x00\x00\x00\x04"},
		{name: "../evil.exe", typeflag: tar.TypeReg, content: "MZ"},
		{name: "app/app.exe", typeflag: tar.TypeReg, content: "second copy"},
	}, false)

	entries, format, err := List(writeFixture(t, "app.zip", data))
	require.NoError(t, err)
	require.Equal(t, Format{Container: ContainerZip}, format)
	require.Len(t, entries, 3)

	require.Equal(t, bareEntry, entries[0].Path)
	require.True(t, entries[0].IsDir())

	require.Equal(t, "app/app.exe", entries[1].Path)
	require.True(t, entries[1].IsRegular(), "entries without unix bits are still regular files")
	require.Equal(t, []byte("MZ\x90\x00\x03\x00\x00\x00"), entries[1].Head)

	require.Equal(t, "app/x.dll", entries[2].Path)
}

func TestListErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		data      []byte
		expectErr error
	}{
		{"notes.txt", []byte("just some text\n"), ErrUnknownFormat},
		{"app.rar", []byte("Rar!\x1a\x07\x00"), ErrUnknownFormat},
		{"app.7z", []byte("7z\xbc\xaf\x27\x1c"), ErrUnknownFormat},
		{"empty", []byte{}, ErrUnknownFormat},
		{"app.tar.gz", append([]byte{0x1f, 0x8b}, []byte("this is not gzip data")...), nil},
		{"app.tar", []byte("this is not a tar file at all"), nil},
		{"app.gz", []byte("plain text with a gz extension"), ErrUnknownFormat},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := List(writeFixture(t, tc.name, tc.data))
			require.Error(t, err)
			if tc.expectErr != nil {
				require.ErrorIs(t, err, tc.expectErr)
			}
		})
	}

	t.Run("missing-file", func(t *testing.T) {
		t.Parallel()
		_, _, err := List(filepath.Join(t.TempDir(), "nope.tar.gz"))
		require.Error(t, err)
		require.NotErrorIs(t, err, ErrUnknownFormat)
	})
}

func TestExtract(t *testing.T) {
	t.Parallel()
	fixtures := standardFixtures(t)
	fixtures["app.gz"] = gzipData(t, []byte(binContent))
	fixtures["app.bz2"] = decodeFixture(t, fixtureBz2)

	for name, data := range fixtures {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			archivePath := writeFixture(t, name, data)
			entry := entryBin
			if formatFromExtension(name).Container == "" {
				entry = bareEntry
			}

			dst := filepath.Join(t.TempDir(), "extracted")
			require.NoError(t, Extract(archivePath, entry, dst, 0))
			got, err := os.ReadFile(dst) //nolint:gosec // test path
			require.NoError(t, err)
			require.Equal(t, binContent, string(got))
			if runtime.GOOS != "windows" {
				st, err := os.Stat(dst)
				require.NoError(t, err)
				require.Equal(t, fs.FileMode(0o600), st.Mode().Perm())
			}

			// Extracting again overwrites the previous output
			require.NoError(t, Extract(archivePath, entry, dst, int64(len(binContent))))
			got, err = os.ReadFile(dst) //nolint:gosec // test path
			require.NoError(t, err)
			require.Equal(t, binContent, string(got))
		})
	}
}

func TestExtractErrors(t *testing.T) {
	t.Parallel()
	for name, data := range standardFixtures(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			archivePath := writeFixture(t, name, data)
			for _, tc := range []struct {
				entry     string
				expectErr error
			}{
				{"app-1.0/missing", ErrEntryNotFound},
				{entryDir, ErrNotRegularFile},
				{entryLink, ErrNotRegularFile},
			} {
				dst := filepath.Join(t.TempDir(), "extracted")
				err := Extract(archivePath, tc.entry, dst, 0)
				require.ErrorIs(t, err, tc.expectErr, tc.entry)
				require.NoFileExists(t, dst)
			}
		})
	}

	t.Run("bare-wrong-name", func(t *testing.T) {
		t.Parallel()
		archivePath := writeFixture(t, "app.gz", gzipData(t, []byte(binContent)))
		dst := filepath.Join(t.TempDir(), "extracted")
		require.ErrorIs(t, Extract(archivePath, "other", dst, 0), ErrEntryNotFound)
		require.NoFileExists(t, dst)
	})

	t.Run("unknown-format", func(t *testing.T) {
		t.Parallel()
		archivePath := writeFixture(t, "notes.txt", []byte("text"))
		require.ErrorIs(t, Extract(archivePath, "notes", filepath.Join(t.TempDir(), "x"), 0), ErrUnknownFormat)
	})
}

func TestExtractTooLarge(t *testing.T) {
	t.Parallel()
	const size = 2 << 20 // 2 MiB of zeros compress to almost nothing
	big := make([]byte, size)
	archivePath := writeFixture(t, "big.tar.gz", gzipData(t, buildTar(t, []testEntry{
		{name: "big/blob", typeflag: tar.TypeReg, mode: 0o755, content: string(big)},
	})))

	for _, tc := range []struct {
		name    string
		maxSize int64
		tooBig  bool
	}{
		{"under-cap", 1 << 20, true},
		{"one-byte-short", size - 1, true},
		{"exact-cap", size, false},
		{"over-cap", size + 1, false},
		{"no-cap", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dst := filepath.Join(t.TempDir(), "blob")
			err := Extract(archivePath, "big/blob", dst, tc.maxSize)
			if tc.tooBig {
				require.ErrorIs(t, err, ErrTooLarge)
				require.NoFileExists(t, dst, "partial output must be removed")
				return
			}
			require.NoError(t, err)
			st, err := os.Stat(dst)
			require.NoError(t, err)
			require.Equal(t, int64(size), st.Size())
		})
	}
}

func TestExtractCorruptZip(t *testing.T) {
	t.Parallel()
	data := buildZip(t, standardLayout(), true)
	// Flip a byte inside the compressed payload of the first file so the
	// checksum verification fails at the end of the entry.
	data[len("PK\x03\x04")+26+len(entryDir+"/")+1] ^= 0xff
	archivePath := writeFixture(t, "app.zip", data)
	dst := filepath.Join(t.TempDir(), "extracted")
	if err := Extract(archivePath, entryBin, dst, 0); err != nil {
		require.NoFileExists(t, dst)
	}
}

func TestFormatString(t *testing.T) {
	t.Parallel()
	require.Equal(t, "tar+gzip", Format{Container: ContainerTar, Compression: CompressionGzip}.String())
	require.Equal(t, "zip", Format{Container: ContainerZip}.String())
	require.Equal(t, "xz", Format{Compression: CompressionXz}.String())
	require.Empty(t, Format{}.String())
}

func TestCleanName(t *testing.T) {
	t.Parallel()
	const nested = "dir/app"
	for _, tc := range []struct {
		name   string
		expect string
		ok     bool
	}{
		{bareEntry, bareEntry, true},
		{"./app", bareEntry, true},
		{nested, nested, true},
		{"dir//app/", nested, true},
		{`dir\app`, nested, true},
		{"dir/./app", nested, true},
		{"dir/../app", bareEntry, true},
		{"", "", false},
		{".", "", false},
		{"./", "", false},
		{"..", "", false},
		{"../app", "", false},
		{"dir/../../app", "", false},
		{"/app", "", false},
		{`\app`, "", false},
		{`C:\app`, "C:/app", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := cleanName(tc.name)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.expect, got)
		})
	}
}
