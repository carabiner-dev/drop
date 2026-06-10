// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package drop

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/carabiner-dev/drop/pkg/github"
	"github.com/carabiner-dev/drop/pkg/system"
)

type fakeRunner struct {
	paths     map[string]bool
	run       [][]string
	silent    [][]string
	runErr    error
	silentErr error
}

func (f *fakeRunner) Run(argv []string) error {
	f.run = append(f.run, argv)
	return f.runErr
}

func (f *fakeRunner) RunSilent(argv []string) error {
	f.silent = append(f.silent, argv)
	return f.silentErr
}

func (f *fakeRunner) LookPath(file string) (string, error) {
	if f.paths[file] {
		return "/usr/bin/" + file, nil
	}
	return "", errors.New("executable file not found in $PATH")
}

func testInstallable() *github.Installable {
	return &github.Installable{
		Name: "drop",
		Variants: []*github.Asset{
			{Name: "drop-linux-amd64", Os: "linux", Arch: "amd64"},
			{Name: "drop-linux-arm64", Os: "linux", Arch: "arm64"},
			{Name: "drop_1.0.0_amd64.deb", Os: "linux", Arch: "amd64"},
			{Name: "drop-1.0.0-1.x86_64.rpm", Os: "linux", Arch: "amd64"},
			{Name: "drop-linux-amd64.tar.gz", Os: "linux", Arch: "amd64"},
			{Name: "drop-darwin-arm64.dmg", Os: "darwin", Arch: "arm64"},
			{Name: "drop-windows-amd64.exe", Os: "windows", Arch: "amd64"},
		},
	}
}

func TestClassifyInstallCandidates(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		os          string
		arch        string
		pkgFormat   string
		binaryName  string // expected variant filename, "" = no binary
		installName string
		pkgName     string // expected variant filename, "" = no package
		hasArchives bool
		hasOtherPkg bool
	}{
		{
			name: "linux-rpm", os: "linux", arch: "amd64", pkgFormat: "rpm",
			binaryName: "drop-linux-amd64", installName: "drop",
			pkgName: "drop-1.0.0-1.x86_64.rpm", hasArchives: true, hasOtherPkg: true,
		},
		{
			name: "linux-deb", os: "linux", arch: "amd64", pkgFormat: "deb",
			binaryName: "drop-linux-amd64", installName: "drop",
			pkgName: "drop_1.0.0_amd64.deb", hasArchives: true, hasOtherPkg: true,
		},
		{
			name: "linux-arm64-binary-only", os: "linux", arch: "arm64", pkgFormat: "rpm",
			binaryName: "drop-linux-arm64", installName: "drop",
		},
		{
			name: "windows-exe", os: "windows", arch: "amd64", pkgFormat: "",
			binaryName: "drop-windows-amd64.exe", installName: "drop.exe",
		},
		{
			name: "darwin-dmg-unsupported", os: "darwin", arch: "arm64", pkgFormat: "",
			hasOtherPkg: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cands := classifyInstallCandidates(testInstallable(), tc.os, tc.arch, tc.pkgFormat)

			if tc.binaryName == "" {
				require.Nil(t, cands.Binary)
			} else {
				require.NotNil(t, cands.Binary)
				require.Equal(t, tc.binaryName, cands.Binary.Asset.GetName())
				require.Equal(t, tc.installName, cands.Binary.InstallName)
				require.Equal(t, ArtifactBinary, cands.Binary.Kind)
			}

			if tc.pkgName == "" {
				require.Nil(t, cands.Package)
			} else {
				require.NotNil(t, cands.Package)
				require.Equal(t, tc.pkgName, cands.Package.Asset.GetName())
				require.Equal(t, tc.pkgFormat, cands.Package.PackageFormat)
				require.Equal(t, ArtifactPackage, cands.Package.Kind)
			}

			require.Equal(t, tc.hasArchives, cands.HasArchives)
			require.Equal(t, tc.hasOtherPkg, cands.HasOtherPkg)
		})
	}
}

func TestDecideArtifact(t *testing.T) {
	t.Parallel()
	binary := &InstallArtifact{Kind: ArtifactBinary, InstallName: "drop"}
	pkg := &InstallArtifact{Kind: ArtifactPackage, PackageFormat: "rpm", InstallName: "drop"}

	pickPackage := func(cands []*InstallArtifact) (*InstallArtifact, error) {
		require.Len(t, cands, 2)
		return cands[1], nil
	}

	for _, tc := range []struct {
		name         string
		cands        *installCandidates
		downloadType string
		selector     ArtifactSelector
		installed    bool
		expect       *InstallArtifact
		expectErr    error
	}{
		{name: "forced-binary", cands: &installCandidates{Binary: binary, Package: pkg}, downloadType: "b", expect: binary},
		{name: "forced-package", cands: &installCandidates{Binary: binary, Package: pkg}, downloadType: "p", expect: pkg},
		{name: "forced-binary-missing", cands: &installCandidates{Package: pkg}, downloadType: "b", expectErr: ErrNoInstallableArtifact},
		{name: "forced-package-missing", cands: &installCandidates{Binary: binary}, downloadType: "p", expectErr: ErrNoInstallableArtifact},
		{name: "only-binary", cands: &installCandidates{Binary: binary}, expect: binary},
		{name: "only-package", cands: &installCandidates{Package: pkg}, expect: pkg},
		{name: "none", cands: &installCandidates{}, expectErr: ErrNoInstallableArtifact},
		{name: "only-archives", cands: &installCandidates{HasArchives: true}, expectErr: ErrOnlyArchives},
		{name: "both-already-installed", cands: &installCandidates{Binary: binary, Package: pkg}, installed: true, expect: pkg},
		{name: "both-selector", cands: &installCandidates{Binary: binary, Package: pkg}, selector: pickPackage, expect: pkg},
		{name: "both-no-selector-defaults-binary", cands: &installCandidates{Binary: binary, Package: pkg}, expect: binary},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			opts := &GetOptions{DownloadType: tc.downloadType, Selector: tc.selector}
			res, err := decideArtifact(tc.cands, opts, func(string) bool { return tc.installed })
			if tc.expectErr != nil {
				require.ErrorIs(t, err, tc.expectErr)
				return
			}
			require.NoError(t, err)
			require.Same(t, tc.expect, res)
		})
	}
}

func TestBuildPackageInstallCmd(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		format    string
		path      string
		sudo      bool
		paths     map[string]bool
		expect    []string
		expectErr bool
	}{
		{
			name: "rpm-dnf-sudo", format: "rpm", path: "/tmp/d/drop.rpm", sudo: true,
			paths:  map[string]bool{"dnf": true, "yum": true, "sudo": true},
			expect: []string{"sudo", "dnf", "install", "-y", "/tmp/d/drop.rpm"},
		},
		{
			name: "rpm-yum-fallback", format: "rpm", path: "/tmp/d/drop.rpm", sudo: false,
			paths:  map[string]bool{"yum": true},
			expect: []string{"yum", "install", "-y", "/tmp/d/drop.rpm"},
		},
		{
			name: "rpm-rpm-fallback", format: "rpm", path: "/tmp/d/drop.rpm", sudo: false,
			paths:  map[string]bool{"rpm": true},
			expect: []string{"rpm", "-Uvh", "/tmp/d/drop.rpm"},
		},
		{
			name: "rpm-no-manager", format: "rpm", path: "/tmp/d/drop.rpm",
			paths: map[string]bool{}, expectErr: true,
		},
		{
			name: "deb-apt", format: "deb", path: "/tmp/d/drop.deb", sudo: true,
			paths:  map[string]bool{"apt": true, "sudo": true},
			expect: []string{"sudo", "apt", "install", "-y", "/tmp/d/drop.deb"},
		},
		{
			name: "deb-dpkg-fallback", format: "deb", path: "/tmp/d/drop.deb", sudo: false,
			paths:  map[string]bool{"dpkg": true},
			expect: []string{"dpkg", "-i", "/tmp/d/drop.deb"},
		},
		{
			name: "apk", format: "apk", path: "/tmp/d/drop.apk", sudo: false,
			paths:  map[string]bool{"apk": true},
			expect: []string{"apk", "add", "--allow-untrusted", "/tmp/d/drop.apk"},
		},
		{
			name: "sudo-missing", format: "rpm", path: "/tmp/d/drop.rpm", sudo: true,
			paths: map[string]bool{"dnf": true}, expectErr: true,
		},
		{
			name: "unsupported-format", format: "msi", path: "/tmp/d/drop.msi",
			paths: map[string]bool{}, expectErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runner := &fakeRunner{paths: tc.paths}
			argv, err := buildPackageInstallCmd(tc.format, tc.path, tc.sudo, runner.LookPath)
			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expect, argv)
		})
	}
}

func TestBuildPackageQueryCmd(t *testing.T) {
	t.Parallel()
	allTools := map[string]bool{"rpm": true, "dpkg": true, "apk": true}
	for _, tc := range []struct {
		name      string
		format    string
		paths     map[string]bool
		expect    []string
		expectErr bool
	}{
		{name: "rpm", format: "rpm", paths: allTools, expect: []string{"rpm", "-q", "drop"}},
		{name: "deb", format: "deb", paths: allTools, expect: []string{"dpkg", "-s", "drop"}},
		{name: "apk", format: "apk", paths: allTools, expect: []string{"apk", "info", "-e", "drop"}},
		{name: "tool-missing", format: "rpm", paths: map[string]bool{}, expectErr: true},
		{name: "unsupported", format: "dmg", paths: allTools, expectErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runner := &fakeRunner{paths: tc.paths}
			argv, err := buildPackageQueryCmd(tc.format, "drop", runner.LookPath)
			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expect, argv)
		})
	}
}

func TestInstallAssetBinary(t *testing.T) {
	t.Parallel()
	writeSource := func(t *testing.T) string {
		t.Helper()
		src := filepath.Join(t.TempDir(), "drop-linux-amd64")
		require.NoError(t, os.WriteFile(src, []byte("#!/bin/true"), 0o600))
		return src
	}
	artifact := &InstallArtifact{Kind: ArtifactBinary, InstallName: "drop"}
	info := &system.Info{Os: "linux", Arch: "amd64"}

	t.Run("writable-dir", func(t *testing.T) {
		t.Parallel()
		binDir := t.TempDir()
		runner := &fakeRunner{paths: map[string]bool{"sudo": true}}
		di := &defaultImplementation{runner: runner}
		opts := &GetOptions{BinDir: binDir}
		opts.Listener = &NoopListener{}

		require.NoError(t, di.InstallAsset(opts, info, artifact, writeSource(t)))

		target := filepath.Join(binDir, "drop")
		st, err := os.Stat(target)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o755), st.Mode().Perm())
		require.Empty(t, runner.run, "no command should run when the dir is writable")
	})

	t.Run("non-writable-dir-uses-sudo", func(t *testing.T) {
		t.Parallel()
		if os.Geteuid() == 0 {
			t.Skip("running as root, no dir is non-writable")
		}
		binDir := filepath.Join(t.TempDir(), "bin")
		require.NoError(t, os.Mkdir(binDir, 0o555)) //nolint:gosec // intentionally non-writable
		runner := &fakeRunner{paths: map[string]bool{"sudo": true}}
		di := &defaultImplementation{runner: runner}
		opts := &GetOptions{BinDir: binDir}
		opts.Listener = &NoopListener{}

		src := writeSource(t)
		require.NoError(t, di.InstallAsset(opts, info, artifact, src))
		require.Equal(t, [][]string{
			{"sudo", "install", "-m", "0755", src, filepath.Join(binDir, "drop")},
		}, runner.run)
	})

	t.Run("non-writable-dir-no-sudo", func(t *testing.T) {
		t.Parallel()
		if os.Geteuid() == 0 {
			t.Skip("running as root, no dir is non-writable")
		}
		binDir := filepath.Join(t.TempDir(), "bin")
		require.NoError(t, os.Mkdir(binDir, 0o555)) //nolint:gosec // intentionally non-writable
		runner := &fakeRunner{paths: map[string]bool{}}
		di := &defaultImplementation{runner: runner}
		opts := &GetOptions{BinDir: binDir}
		opts.Listener = &NoopListener{}

		require.Error(t, di.InstallAsset(opts, info, artifact, writeSource(t)))
		require.Empty(t, runner.run)
	})
}

func TestInstallAssetPackage(t *testing.T) {
	t.Parallel()
	runner := &fakeRunner{paths: map[string]bool{"dnf": true, "sudo": true}}
	di := &defaultImplementation{runner: runner}
	opts := &GetOptions{}
	opts.Listener = &NoopListener{}
	artifact := &InstallArtifact{
		Kind: ArtifactPackage, PackageFormat: "rpm", InstallName: "drop",
	}

	require.NoError(t, di.InstallAsset(opts, &system.Info{Os: "linux"}, artifact, "/tmp/d/drop.rpm"))
	require.Len(t, runner.run, 1)
	if os.Geteuid() == 0 {
		require.Equal(t, []string{"dnf", "install", "-y", "/tmp/d/drop.rpm"}, runner.run[0])
	} else {
		require.Equal(t, []string{"sudo", "dnf", "install", "-y", "/tmp/d/drop.rpm"}, runner.run[0])
	}
}

func TestPackageInstalled(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		format    string
		silentErr error
		expect    bool
	}{
		{name: "installed", format: "rpm", expect: true},
		{name: "not-installed", format: "rpm", silentErr: errors.New("exit 1"), expect: false},
		{name: "no-format", format: "", expect: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runner := &fakeRunner{
				paths:     map[string]bool{"rpm": true},
				silentErr: tc.silentErr,
			}
			di := &defaultImplementation{runner: runner}
			require.Equal(t, tc.expect, di.packageInstalled(tc.format, "drop"))
		})
	}
}

func TestDownloadAssetToTmp(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("artifact-data")) //nolint:errcheck
	}))
	defer srv.Close()

	di := &defaultImplementation{}
	opts := &GetOptions{TransferTimeOut: 10}
	opts.computedFilename = "drop-1.0.0-1.x86_64.rpm"
	opts.Listener = &NoopListener{}
	asset := &github.Asset{
		Name:        "drop-1.0.0-1.x86_64.rpm",
		DownloadURL: srv.URL + "/drop-1.0.0-1.x86_64.rpm",
	}

	path, err := di.DownloadAssetToTmp(opts, asset)
	require.NoError(t, err)
	defer os.RemoveAll(filepath.Dir(path)) //nolint:errcheck

	require.Equal(t, "drop-1.0.0-1.x86_64.rpm", filepath.Base(path), "tmp file must keep the package extension")
	data, err := os.ReadFile(path) //nolint:gosec // path is a test-controlled tmp file
	require.NoError(t, err)
	require.Equal(t, "artifact-data", string(data))
}
