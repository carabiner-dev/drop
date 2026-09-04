// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package drop

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/carabiner-dev/drop/pkg/archive"
	"github.com/carabiner-dev/drop/pkg/system"
)

// Fake file contents starting with the magic of each executable format
const (
	elfContent   = "\x7fELF\x02\x01\x01\x00fake linux binary\n"
	peContent    = "MZ\x90\x00\x03\x00\x00\x00fake windows binary\n"
	machoContent = "\xcf\xfa\xed\xfe\x07\x00\x00\x01fake mac binary\n"
	textContent  = "#!/bin/sh\necho hello\n"
	appleDouble  = "\x00\x05\x16\x07\x00\x02\x00\x00"
)

// Names used across the selection and install tests
const (
	appName      = "app"
	appExe       = "app.exe"
	appNested    = "app/app"
	appBinEntry  = "app-1.0/bin/app"
	appVersioned = "app-1.0.0"
	appTarGz     = "app_1.0_linux_amd64.tar.gz"
	fooName      = "foo"
	hugoName     = "hugo"
	hugoExtended = "hugo_extended"
	hugoTarGz    = "hugo_extended_0.1_linux-amd64.tar.gz"
	k8sClient    = "kubernetes-client"
	kubeadmEntry = "bin/kubeadm"
	kubectlEntry = "bin/kubectl"
	licenseFile  = "LICENSE"
	licenseText  = "MIT"
	readmeFile   = "README"
	bareGz       = "app-linux-amd64.gz"
	bareEntry    = "app-linux-amd64"
	toolsName    = "tools"
	toolsTarGz   = "tools_1.0_linux_amd64.tar.gz"
	toolsBar     = "tools/bar"
)

// recorder is a listener that keeps the events it receives
type recorder struct {
	events []*Event
}

func (r *recorder) HandleEvent(event *Event) {
	r.events = append(r.events, event)
}

func (r *recorder) find(object, verb string) *Event {
	for _, e := range r.events {
		if e.Object == object && e.Verb == verb {
			return e
		}
	}
	return nil
}

// regular, dir and symlink build archive entries for the selection tests
func regular(p, content string) archive.Entry {
	head := []byte(content)
	if len(head) > system.ExecutableHeadSize {
		head = head[:system.ExecutableHeadSize]
	}
	return archive.Entry{Path: p, Mode: 0o755, Head: head}
}

func dir(p string) archive.Entry {
	return archive.Entry{Path: p, Mode: fs.ModeDir | 0o755}
}

func symlink(p, target string) archive.Entry {
	return archive.Entry{Path: p, Mode: fs.ModeSymlink | 0o777, Linkname: target}
}

func TestIsSharedLibrary(t *testing.T) {
	t.Parallel()
	for name, expect := range map[string]bool{
		appName:          false,
		appExe:           false,
		"libapp.so":      true,
		"libapp.so.1":    true,
		"libapp.so.1.2":  true,
		"libapp.dylib":   true,
		"APP.DLL":        true,
		"libapp.a":       true,
		"app.lib":        true,
		"resolver":       false,
		"something.sock": false,
	} {
		require.Equal(t, expect, isSharedLibrary(name), name)
	}
}

func TestArchiveCandidates(t *testing.T) {
	t.Parallel()
	entries := []archive.Entry{
		dir(appName),
		regular(appNested, elfContent),
		regular("app/app.exe", peContent),
		regular("app/App.app", machoContent),
		regular("app/run.sh", textContent),
		regular("app/libapp.so", elfContent),
		regular("app/helper.dll", peContent),
		regular("app/README.md", "# app\n"),
		regular("app/tiny", "\x7fE"),
		regular("__MACOSX/._app", appleDouble),
		regular("app/noext", peContent),
		symlink("app/link", appName),
	}

	for _, tc := range []struct {
		os     string
		expect []string
	}{
		{system.OSLinux, []string{appNested}},
		{system.OSFreeBSD, []string{appNested}},
		{system.OSDarwin, []string{"app/App.app"}},
		{system.OSWindows, []string{"app/app.exe"}},
		{"", []string{appNested, "app/app.exe", "app/App.app", "app/noext"}},
	} {
		t.Run(tc.os, func(t *testing.T) {
			t.Parallel()
			got := []string{}
			for _, c := range archiveCandidates(entries, tc.os) {
				got = append(got, c.Path)
			}
			require.Equal(t, tc.expect, got)
		})
	}
}

func TestSelectArchiveEntry(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		entries   []archive.Entry
		wanted    string
		pinned    string
		required  bool
		os        string
		expect    string
		expectErr error
	}{
		{
			name:    "nested-dir",
			entries: []archive.Entry{dir("bat-v0.24"), regular("bat-v0.24/bat", elfContent), regular("bat-v0.24/README.md", "# bat\n")},
			wanted:  "bat", os: system.OSLinux, expect: "bat-v0.24/bat",
		},
		{
			name:    "bin-subdir",
			entries: []archive.Entry{regular("gh_2.40/bin/gh", elfContent), regular("gh_2.40/share/man/gh.1", ".TH gh\n")},
			wanted:  "gh", os: system.OSLinux, expect: "gh_2.40/bin/gh",
		},
		{
			name:    "shallowest-wins",
			entries: []archive.Entry{regular("examples/foo", elfContent), regular(fooName, elfContent), regular("a/foo", elfContent)},
			wanted:  fooName, os: system.OSLinux, expect: fooName,
		},
		{
			name:    "linux-ignores-exe",
			entries: []archive.Entry{regular("foo.exe", peContent), regular(fooName, elfContent)},
			wanted:  fooName, os: system.OSLinux, expect: fooName,
		},
		{
			name:    "windows-picks-exe",
			entries: []archive.Entry{regular(fooName, elfContent), regular("foo.exe", peContent)},
			wanted:  fooName, os: system.OSWindows, expect: "foo.exe",
		},
		{
			name:    "windows-case-insensitive",
			entries: []archive.Entry{regular("Hugo.EXE", peContent)},
			wanted:  hugoName, os: system.OSWindows, expect: "Hugo.EXE",
		},
		{
			name:    "symlink-resolves",
			entries: []archive.Entry{regular("foo-1.2", elfContent), symlink(fooName, "foo-1.2")},
			wanted:  fooName, os: system.OSLinux, expect: "foo-1.2",
		},
		{
			name:    "symlink-in-subdir",
			entries: []archive.Entry{regular("app/lib/foo-1.2", elfContent), symlink("app/bin/foo", "../lib/foo-1.2")},
			wanted:  fooName, os: system.OSLinux, expect: "app/lib/foo-1.2",
		},
		{
			name:    "symlink-chain",
			entries: []archive.Entry{regular("foo-1.2", elfContent), symlink("foo-1", "foo-1.2"), symlink(fooName, "foo-1")},
			wanted:  fooName, os: system.OSLinux, expect: "foo-1.2",
		},
		{
			name:    "symlink-loop",
			entries: []archive.Entry{symlink(fooName, "bar"), symlink("bar", fooName), regular("other", elfContent)},
			wanted:  fooName, os: system.OSLinux, expectErr: ErrNoMatchingArchiveEntry,
		},
		{
			name:    "symlink-escaping",
			entries: []archive.Entry{symlink(fooName, "../../bin/foo"), regular("foo-1.2", elfContent)},
			wanted:  fooName, os: system.OSLinux, expectErr: ErrNoMatchingArchiveEntry,
		},
		{
			name:    "symlink-absolute",
			entries: []archive.Entry{symlink(fooName, "/usr/bin/foo"), regular("foo-1.2", elfContent)},
			wanted:  fooName, os: system.OSLinux, expectErr: ErrNoMatchingArchiveEntry,
		},
		{
			name:    "dir-and-file-with-the-name",
			entries: []archive.Entry{dir(fooName), regular("foo/foo", elfContent)},
			wanted:  fooName, os: system.OSLinux, expect: "foo/foo",
		},
		{
			name:    "text-file-with-the-name-is-not-the-binary",
			entries: []archive.Entry{regular(fooName, textContent), regular("bin/foo", elfContent)},
			wanted:  fooName, os: system.OSLinux, expect: "bin/foo",
		},
		{
			name:    "wrapper-script-only",
			entries: []archive.Entry{regular(fooName, textContent), regular(readmeFile, "readme")},
			wanted:  fooName, os: system.OSLinux, expectErr: ErrNoBinaryInArchive,
		},
		{
			name:    "libraries-are-not-candidates",
			entries: []archive.Entry{regular(fooName, elfContent), regular("libfoo.so", elfContent), regular("libfoo.so.1", elfContent)},
			wanted:  "bar", os: system.OSLinux, expectErr: ErrNoMatchingArchiveEntry,
		},
		{
			name:    "dll-is-not-a-candidate",
			entries: []archive.Entry{regular("foo.exe", peContent), regular("libwinpthread-1.dll", peContent)},
			wanted:  "bar", os: system.OSWindows, expectErr: ErrNoMatchingArchiveEntry,
		},
		{
			name:    "several-executables-none-matching",
			entries: []archive.Entry{regular("kubernetes/client/bin/kubectl", elfContent), regular("kubernetes/client/bin/kubeadm", elfContent)},
			wanted:  k8sClient, os: system.OSLinux, expectErr: ErrNoMatchingArchiveEntry,
		},
		{
			name:    "macos-resource-fork",
			entries: []archive.Entry{regular("__MACOSX/._foo", appleDouble), regular(fooName, machoContent)},
			wanted:  fooName, os: system.OSDarwin, expect: fooName,
		},
		{
			name:    "wrong-platform-binary",
			entries: []archive.Entry{regular(fooName, peContent)},
			wanted:  fooName, os: system.OSLinux, expectErr: ErrNoBinaryInArchive,
		},
		{
			name:    "empty",
			entries: []archive.Entry{},
			wanted:  fooName, os: system.OSLinux, expectErr: ErrNoBinaryInArchive,
		},
		{
			name:    "pinned-present",
			entries: []archive.Entry{regular("tools/kubectl", elfContent), regular("tools/kubeadm", elfContent)},
			wanted:  k8sClient, pinned: "tools/kubeadm", os: system.OSLinux, expect: "tools/kubeadm",
		},
		{
			name:    "pinned-beats-name-match",
			entries: []archive.Entry{regular(fooName, elfContent), regular("foo-extended", elfContent)},
			wanted:  fooName, pinned: "foo-extended", os: system.OSLinux, expect: "foo-extended",
		},
		{
			name:    "pinned-missing-falls-back-to-name",
			entries: []archive.Entry{regular("v2/foo", elfContent)},
			wanted:  fooName, pinned: "v1/foo", os: system.OSLinux, expect: "v2/foo",
		},
		{
			name:    "pinned-not-executable-is-ignored",
			entries: []archive.Entry{regular(readmeFile, "readme"), regular(fooName, elfContent)},
			wanted:  fooName, pinned: readmeFile, os: system.OSLinux, expect: fooName,
		},
		{
			name:    "pinned-missing-no-match",
			entries: []archive.Entry{regular("kubectl", elfContent)},
			wanted:  k8sClient, pinned: kubectlEntry, os: system.OSLinux, expectErr: ErrNoMatchingArchiveEntry,
		},
		{
			name:    "required-present",
			entries: []archive.Entry{regular(fooName, elfContent), regular(toolsBar, elfContent)},
			wanted:  fooName, pinned: toolsBar, required: true, os: system.OSLinux, expect: toolsBar,
		},
		{
			name:    "required-missing-does-not-fall-back",
			entries: []archive.Entry{regular(fooName, elfContent)},
			wanted:  fooName, pinned: toolsBar, required: true, os: system.OSLinux, expectErr: ErrBadArchiveEntry,
		},
		{
			name:    "required-not-executable",
			entries: []archive.Entry{regular(fooName, elfContent), regular(readmeFile, "readme")},
			wanted:  fooName, pinned: readmeFile, required: true, os: system.OSLinux, expectErr: ErrBadArchiveEntry,
		},
		{
			name:    "required-wrong-platform",
			entries: []archive.Entry{regular(fooName, elfContent), regular(appExe, peContent)},
			wanted:  fooName, pinned: appExe, required: true, os: system.OSLinux, expectErr: ErrBadArchiveEntry,
		},
		{
			name:    "required-symlink-is-not-a-file",
			entries: []archive.Entry{regular(appVersioned, elfContent), symlink(appName, appVersioned)},
			wanted:  appName, pinned: appName, required: true, os: system.OSLinux, expectErr: ErrBadArchiveEntry,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := selectArchiveEntry(tc.entries, &archiveChoice{
				archive: appTarGz, wanted: tc.wanted, pinned: tc.pinned, required: tc.required, targetOS: tc.os,
			})
			if tc.expectErr != nil {
				require.ErrorIs(t, err, tc.expectErr)
				require.Nil(t, got)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Equal(t, tc.expect, got.Path)
			require.True(t, got.IsRegular())
		})
	}
}

// fakeSelector records the prompts it receives and answers with a fixed
// choice or error.
type fakeSelector struct {
	calls      int
	archive    string
	wanted     string
	candidates []string
	choice     string
	err        error
}

func (f *fakeSelector) fn() ArchiveEntrySelector {
	return func(archiveName, wanted string, candidates []string) (string, error) {
		f.calls++
		f.archive, f.wanted, f.candidates = archiveName, wanted, candidates
		return f.choice, f.err
	}
}

func TestSelectArchiveEntrySelector(t *testing.T) {
	t.Parallel()
	single := []archive.Entry{regular(licenseFile, licenseText), regular(hugoName, elfContent)}
	several := []archive.Entry{regular(kubectlEntry, elfContent), regular(kubeadmEntry, elfContent), regular(readmeFile, "readme")}

	for _, tc := range []struct {
		name             string
		entries          []archive.Entry
		wanted           string
		pinned           string
		required         bool
		selector         *fakeSelector
		expect           string
		expectCandidates []string
		expectErr        error
		expectErrText    string
	}{
		{
			name: "single-confirmed", entries: single, wanted: hugoExtended,
			selector: &fakeSelector{choice: hugoName},
			expect:   hugoName, expectCandidates: []string{hugoName},
		},
		{
			name: "single-declined", entries: single, wanted: hugoExtended,
			selector:  &fakeSelector{err: ErrInstallAborted},
			expectErr: ErrInstallAborted, expectCandidates: []string{hugoName},
		},
		{
			name: "several-chosen", entries: several, wanted: k8sClient,
			selector: &fakeSelector{choice: kubeadmEntry},
			expect:   kubeadmEntry, expectCandidates: []string{kubectlEntry, kubeadmEntry},
		},
		{
			name: "several-chosen-first", entries: several, wanted: k8sClient,
			selector: &fakeSelector{choice: kubectlEntry},
			expect:   kubectlEntry, expectCandidates: []string{kubectlEntry, kubeadmEntry},
		},
		{
			name: "choice-outside-the-list", entries: several, wanted: k8sClient,
			selector:      &fakeSelector{choice: readmeFile},
			expectErrText: "not one of the executables", expectCandidates: []string{kubectlEntry, kubeadmEntry},
		},
		{
			name: "selector-failure", entries: several, wanted: k8sClient,
			selector:      &fakeSelector{err: errors.New("terminal closed")},
			expectErrText: "terminal closed", expectCandidates: []string{kubectlEntry, kubeadmEntry},
		},
		{
			name: "no-selector", entries: single, wanted: hugoExtended,
			expectErr: ErrNoMatchingArchiveEntry,
		},
		{
			name: "not-asked-on-name-match", entries: single, wanted: hugoName,
			selector: &fakeSelector{choice: licenseFile},
			expect:   hugoName,
		},
		{
			name: "not-asked-on-pin", entries: several, wanted: k8sClient, pinned: kubeadmEntry,
			selector: &fakeSelector{choice: kubectlEntry},
			expect:   kubeadmEntry,
		},
		{
			name: "not-asked-without-executables", entries: []archive.Entry{regular(readmeFile, "readme")}, wanted: hugoName,
			selector:  &fakeSelector{choice: readmeFile},
			expectErr: ErrNoBinaryInArchive,
		},
		{
			name: "not-asked-on-required-miss", entries: several, wanted: k8sClient, pinned: "bin/missing", required: true,
			selector:  &fakeSelector{choice: kubectlEntry},
			expectErr: ErrBadArchiveEntry,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			choice := &archiveChoice{archive: hugoTarGz, wanted: tc.wanted, pinned: tc.pinned, required: tc.required, targetOS: system.OSLinux}
			if tc.selector != nil {
				choice.selector = tc.selector.fn()
			}

			got, err := selectArchiveEntry(tc.entries, choice)

			if tc.selector != nil {
				if tc.expectCandidates == nil {
					require.Zero(t, tc.selector.calls, "the selector must not be asked")
				} else {
					require.Equal(t, 1, tc.selector.calls)
					require.Equal(t, hugoTarGz, tc.selector.archive)
					require.Equal(t, tc.wanted, tc.selector.wanted)
					require.Equal(t, tc.expectCandidates, tc.selector.candidates)
				}
			}

			switch {
			case tc.expectErr != nil:
				require.ErrorIs(t, err, tc.expectErr)
				require.Nil(t, got)
			case tc.expectErrText != "":
				require.ErrorContains(t, err, tc.expectErrText)
				require.Nil(t, got)
			default:
				require.NoError(t, err)
				require.NotNil(t, got)
				require.Equal(t, tc.expect, got.Path)
			}
		})
	}
}

// tarEntry describes a file to pack in a test archive
type tarEntry struct {
	name     string
	content  string
	linkname string
}

func buildTarGz(t *testing.T, entries []tarEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for _, e := range entries {
		hdr := &tar.Header{Name: e.name, Mode: 0o755, Typeflag: tar.TypeReg, Size: int64(len(e.content))}
		switch {
		case e.linkname != "":
			hdr.Typeflag, hdr.Linkname, hdr.Size = tar.TypeSymlink, e.linkname, 0
		case e.name[len(e.name)-1] == '/':
			hdr.Typeflag, hdr.Size = tar.TypeDir, 0
		}
		require.NoError(t, tw.WriteHeader(hdr))
		if hdr.Typeflag == tar.TypeReg {
			_, err := tw.Write([]byte(e.content))
			require.NoError(t, err)
		}
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

func buildZipFile(t *testing.T, entries []tarEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		w, err := zw.Create(e.name)
		require.NoError(t, err)
		_, err = w.Write([]byte(e.content))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func gzipBytes(t *testing.T, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	_, err := gw.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

func TestInstallAssetArchive(t *testing.T) {
	t.Parallel()
	layout := []tarEntry{
		{name: "app-1.0/"},
		{name: "app-1.0/README.md", content: "# app\n"},
		{name: appBinEntry, content: elfContent},
		{name: "app-1.0/lib/libapp.so", content: elfContent},
	}

	for _, tc := range []struct {
		name         string
		archiveName  string
		data         []byte
		artifactName string
		pinned       string
		required     bool
		selector     ArchiveEntrySelector
		os           string
		expectFile   string // installed filename, empty means an error is expected
		expectEntry  string
		expectErr    error
	}{
		{
			name: "tar-gz", archiveName: appTarGz, data: buildTarGz(t, layout),
			artifactName: appName, os: system.OSLinux, expectFile: appName, expectEntry: appBinEntry,
		},
		{
			name: "zip", archiveName: "app_1.0_linux_amd64.zip", data: buildZipFile(t, layout),
			artifactName: appName, os: system.OSLinux, expectFile: appName, expectEntry: appBinEntry,
		},
		{
			name: "windows-zip", archiveName: "app_1.0_windows_amd64.zip",
			data:         buildZipFile(t, []tarEntry{{name: appExe, content: peContent}, {name: "helper.dll", content: peContent}}),
			artifactName: appName, os: system.OSWindows, expectFile: appExe, expectEntry: appExe,
		},
		{
			name: "bare-gz", archiveName: bareGz, data: gzipBytes(t, elfContent),
			artifactName: appName, os: system.OSLinux, expectFile: appName, expectEntry: "",
		},
		{
			name: "bare-gz-not-a-binary", archiveName: bareGz, data: gzipBytes(t, textContent),
			artifactName: appName, os: system.OSLinux, expectErr: ErrNoBinaryInArchive,
		},
		{
			name: "different-name-needs-a-choice", archiveName: hugoTarGz,
			data:         buildTarGz(t, []tarEntry{{name: hugoName, content: elfContent}, {name: licenseFile, content: licenseText}}),
			artifactName: hugoExtended, os: system.OSLinux, expectErr: ErrNoMatchingArchiveEntry,
		},
		{
			name: "different-name-confirmed", archiveName: hugoTarGz,
			data:         buildTarGz(t, []tarEntry{{name: hugoName, content: elfContent}, {name: licenseFile, content: licenseText}}),
			artifactName: hugoExtended, selector: (&fakeSelector{choice: hugoName}).fn(), os: system.OSLinux,
			expectFile: hugoName, expectEntry: hugoName,
		},
		{
			name: "different-name-declined", archiveName: hugoTarGz,
			data:         buildTarGz(t, []tarEntry{{name: hugoName, content: elfContent}, {name: licenseFile, content: licenseText}}),
			artifactName: hugoExtended, selector: (&fakeSelector{err: ErrInstallAborted}).fn(), os: system.OSLinux,
			expectErr: ErrInstallAborted,
		},
		{
			name: "different-name-pinned", archiveName: hugoTarGz,
			data:         buildTarGz(t, []tarEntry{{name: hugoName, content: elfContent}, {name: licenseFile, content: licenseText}}),
			artifactName: hugoExtended, pinned: hugoName, os: system.OSLinux, expectFile: hugoName, expectEntry: hugoName,
		},
		{
			name: "required-entry", archiveName: toolsTarGz,
			data:         buildTarGz(t, []tarEntry{{name: kubectlEntry, content: elfContent}, {name: kubeadmEntry, content: elfContent}}),
			artifactName: toolsName, pinned: kubectlEntry, required: true, os: system.OSLinux, expectFile: "kubectl", expectEntry: kubectlEntry,
		},
		{
			name: "required-entry-missing", archiveName: toolsTarGz,
			data:         buildTarGz(t, []tarEntry{{name: toolsName, content: elfContent}}),
			artifactName: toolsName, pinned: kubectlEntry, required: true, os: system.OSLinux, expectErr: ErrBadArchiveEntry,
		},
		{
			name: "required-entry-bare-file", archiveName: bareGz, data: gzipBytes(t, elfContent),
			artifactName: appName, pinned: bareEntry, required: true, os: system.OSLinux, expectFile: appName, expectEntry: "",
		},
		{
			name: "required-entry-bare-file-mismatch", archiveName: bareGz, data: gzipBytes(t, elfContent),
			artifactName: appName, pinned: "bin/app", required: true, os: system.OSLinux, expectErr: ErrBadArchiveEntry,
		},
		{
			name: "symlinked-binary", archiveName: appTarGz,
			data:         buildTarGz(t, []tarEntry{{name: appVersioned, content: elfContent}, {name: appName, linkname: appVersioned}}),
			artifactName: appName, os: system.OSLinux, expectFile: appName, expectEntry: appVersioned,
		},
		{
			name: "symlinked-binary-pinned-target", archiveName: appTarGz,
			data:         buildTarGz(t, []tarEntry{{name: appVersioned, content: elfContent}, {name: appName, linkname: appVersioned}}),
			artifactName: appName, pinned: appVersioned, os: system.OSLinux, expectFile: appName, expectEntry: appVersioned,
		},
		{
			name: "pinned-under-another-name", archiveName: toolsTarGz,
			data:         buildTarGz(t, []tarEntry{{name: kubectlEntry, content: elfContent}, {name: kubeadmEntry, content: elfContent}}),
			artifactName: toolsName, pinned: kubeadmEntry, os: system.OSLinux, expectFile: "kubeadm", expectEntry: kubeadmEntry,
		},
		{
			name: "wrong-platform", archiveName: appTarGz, data: buildTarGz(t, layout),
			artifactName: appName, os: system.OSDarwin, expectErr: ErrNoBinaryInArchive,
		},
		{
			name: "corrupt", archiveName: appTarGz, data: []byte("\x1f\x8bnot really gzip"),
			artifactName: appName, os: system.OSLinux, expectErr: nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			downloadDir := t.TempDir()
			archivePath := filepath.Join(downloadDir, tc.archiveName)
			require.NoError(t, os.WriteFile(archivePath, tc.data, 0o600))

			binDir := t.TempDir()
			events := &recorder{}
			opts := &GetOptions{
				BinDir: binDir, OS: tc.os, EntrySelector: tc.selector,
				ArchiveEntry: tc.pinned, ArchiveEntryRequired: tc.required,
			}
			opts.Listener = events
			info := &system.Info{Os: tc.os, Arch: system.ArchAMD64}
			installName := tc.artifactName
			if tc.os == system.OSWindows {
				installName += exeSuffix
			}
			artifact := &InstallArtifact{
				Kind: ArtifactArchive, Name: tc.artifactName, InstallName: installName,
				Asset: nil,
			}
			runner := &fakeRunner{paths: map[string]bool{cmdSudo: true}}
			di := &defaultImplementation{runner: runner}

			err := di.InstallAsset(opts, info, artifact, archivePath)
			if tc.expectFile == "" {
				require.Error(t, err)
				if tc.expectErr != nil {
					require.ErrorIs(t, err, tc.expectErr)
				}
				require.NoDirExists(t, filepath.Join(downloadDir, extractDirName), "nothing is extracted when no entry is chosen")
				entries, readErr := os.ReadDir(binDir)
				require.NoError(t, readErr)
				require.Empty(t, entries)
				return
			}
			require.NoError(t, err)

			installed := filepath.Join(binDir, tc.expectFile)
			got, err := os.ReadFile(installed) //nolint:gosec // test path
			require.NoError(t, err)
			require.Contains(t, []string{elfContent, peContent}, string(got))
			if runtime.GOOS != system.OSWindows {
				st, err := os.Stat(installed)
				require.NoError(t, err)
				require.Equal(t, fs.FileMode(0o755), st.Mode().Perm())
			}
			require.Empty(t, runner.run, "no command runs when the bin dir is writable")

			require.Equal(t, tc.expectEntry, artifact.ArchiveEntry)
			require.Equal(t, tc.expectFile, artifact.InstallName)
			require.Equal(t, tc.artifactName, artifact.Name, "the installable name never changes")

			// The extracted copy stays in the download dir for the caller to clean
			unpacked, err := os.ReadDir(filepath.Join(downloadDir, extractDirName))
			require.NoError(t, err)
			require.Len(t, unpacked, 1)

			running := events.find(EventObjectArchive, EventVerbRunning)
			require.NotNil(t, running)
			require.Equal(t, tc.archiveName, running.GetDataField(dataKeyArchive))
			require.NotEmpty(t, running.GetDataField(dataKeyFormat))
			require.NotNil(t, events.find(EventObjectArchive, EventVerbDone))
			require.NotNil(t, events.find(EventObjectInstall, EventVerbDone))
		})
	}
}
