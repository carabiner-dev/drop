// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package drop

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/carabiner-dev/drop/pkg/github"
	"github.com/carabiner-dev/drop/pkg/inventory"
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

const (
	testAppName = "drop"
	testBinFile = "drop-linux-amd64"
	testRPMFile = "drop-1.0.0-1.x86_64.rpm"
	testRPMPath = "/tmp/d/drop.rpm"
	testDebPath = "/tmp/d/drop.deb"
)

func testInstallable() *github.Installable {
	return &github.Installable{
		Name: testAppName,
		Variants: []*github.Asset{
			{Name: testBinFile, Os: system.OSLinux, Arch: system.ArchAMD64},
			{Name: "drop-linux-arm64", Os: system.OSLinux, Arch: system.ArchArm64},
			{Name: "drop_1.0.0_amd64.deb", Os: system.OSLinux, Arch: system.ArchAMD64},
			{Name: testRPMFile, Os: system.OSLinux, Arch: system.ArchAMD64},
			{Name: "drop-linux-amd64.tar.gz", Os: system.OSLinux, Arch: system.ArchAMD64},
			{Name: "drop-darwin-arm64.dmg", Os: system.OSDarwin, Arch: system.ArchArm64},
			{Name: "drop-windows-amd64.exe", Os: system.OSWindows, Arch: system.ArchAMD64},
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
			name: "linux-rpm", os: system.OSLinux, arch: system.ArchAMD64, pkgFormat: system.PackageRPM,
			binaryName: testBinFile, installName: testAppName,
			pkgName: testRPMFile, hasArchives: true, hasOtherPkg: true,
		},
		{
			name: "linux-deb", os: system.OSLinux, arch: system.ArchAMD64, pkgFormat: system.PackageDeb,
			binaryName: testBinFile, installName: testAppName,
			pkgName: "drop_1.0.0_amd64.deb", hasArchives: true, hasOtherPkg: true,
		},
		{
			name: "linux-arm64-binary-only", os: system.OSLinux, arch: system.ArchArm64, pkgFormat: system.PackageRPM,
			binaryName: "drop-linux-arm64", installName: testAppName,
		},
		{
			name: "windows-exe", os: system.OSWindows, arch: system.ArchAMD64, pkgFormat: "",
			binaryName: "drop-windows-amd64.exe", installName: "drop.exe",
		},
		{
			name: "darwin-dmg-unsupported", os: system.OSDarwin, arch: system.ArchArm64, pkgFormat: "",
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
	binary := &InstallArtifact{Kind: ArtifactBinary, InstallName: testAppName}
	pkg := &InstallArtifact{Kind: ArtifactPackage, PackageFormat: system.PackageRPM, InstallName: testAppName}

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

func mustAbs(t *testing.T, path string) string {
	t.Helper()
	abs, err := filepath.Abs(path)
	require.NoError(t, err)
	return abs
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
			name: "rpm-dnf-sudo", format: system.PackageRPM, path: testRPMPath, sudo: true,
			paths:  map[string]bool{cmdDnf: true, cmdYum: true, cmdSudo: true},
			expect: []string{cmdSudo, cmdDnf, verbInstall, "-y", testRPMPath},
		},
		{
			name: "rpm-yum-fallback", format: system.PackageRPM, path: testRPMPath, sudo: false,
			paths:  map[string]bool{cmdYum: true},
			expect: []string{cmdYum, verbInstall, "-y", testRPMPath},
		},
		{
			name: "rpm-rpm-fallback", format: system.PackageRPM, path: testRPMPath, sudo: false,
			paths:  map[string]bool{system.PackageRPM: true},
			expect: []string{system.PackageRPM, "-Uvh", testRPMPath},
		},
		{
			name: "rpm-no-manager", format: system.PackageRPM, path: testRPMPath,
			paths: map[string]bool{}, expectErr: true,
		},
		{
			// deb paths go through filepath.Abs (apt requires a path to
			// install local files), absolutize the expectation too
			name: "deb-apt", format: system.PackageDeb, path: testDebPath, sudo: true,
			paths:  map[string]bool{"apt": true, cmdSudo: true},
			expect: []string{cmdSudo, "apt", verbInstall, "-y", mustAbs(t, testDebPath)},
		},
		{
			name: "deb-dpkg-fallback", format: system.PackageDeb, path: testDebPath, sudo: false,
			paths:  map[string]bool{cmdDpkg: true},
			expect: []string{cmdDpkg, "-i", mustAbs(t, testDebPath)},
		},
		{
			name: system.PackageApk, format: system.PackageApk, path: "/tmp/d/drop.apk", sudo: false,
			paths:  map[string]bool{system.PackageApk: true},
			expect: []string{system.PackageApk, "add", "--allow-untrusted", "/tmp/d/drop.apk"},
		},
		{
			name: "sudo-missing", format: system.PackageRPM, path: testRPMPath, sudo: true,
			paths: map[string]bool{cmdDnf: true}, expectErr: true,
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
	allTools := map[string]bool{system.PackageRPM: true, cmdDpkg: true, system.PackageApk: true}
	for _, tc := range []struct {
		name      string
		format    string
		paths     map[string]bool
		expect    []string
		expectErr bool
	}{
		{name: system.PackageRPM, format: system.PackageRPM, paths: allTools, expect: []string{system.PackageRPM, "-q", testAppName}},
		{name: system.PackageDeb, format: system.PackageDeb, paths: allTools, expect: []string{cmdDpkg, "-s", testAppName}},
		{name: system.PackageApk, format: system.PackageApk, paths: allTools, expect: []string{system.PackageApk, "info", "-e", testAppName}},
		{name: "tool-missing", format: system.PackageRPM, paths: map[string]bool{}, expectErr: true},
		{name: "unsupported", format: "dmg", paths: allTools, expectErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runner := &fakeRunner{paths: tc.paths}
			argv, err := buildPackageQueryCmd(tc.format, testAppName, runner.LookPath)
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
		src := filepath.Join(t.TempDir(), testBinFile)
		require.NoError(t, os.WriteFile(src, []byte("#!/bin/true"), 0o600))
		return src
	}
	artifact := &InstallArtifact{Kind: ArtifactBinary, InstallName: testAppName}
	info := &system.Info{Os: system.OSLinux, Arch: system.ArchAMD64}

	t.Run("writable-dir", func(t *testing.T) {
		t.Parallel()
		binDir := t.TempDir()
		runner := &fakeRunner{paths: map[string]bool{cmdSudo: true}}
		di := &defaultImplementation{runner: runner}
		opts := &GetOptions{BinDir: binDir}
		opts.Listener = &NoopListener{}

		require.NoError(t, di.InstallAsset(opts, info, artifact, writeSource(t)))

		target := filepath.Join(binDir, testAppName)
		st, err := os.Stat(target)
		require.NoError(t, err)
		if runtime.GOOS != system.OSWindows {
			// windows does not preserve unix permission bits
			require.Equal(t, os.FileMode(0o755), st.Mode().Perm())
		}
		require.Empty(t, runner.run, "no command should run when the dir is writable")
	})

	t.Run("non-writable-dir-uses-sudo", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == system.OSWindows {
			t.Skip("directory permissions are not enforced on windows")
		}
		if os.Geteuid() == 0 {
			t.Skip("running as root, no dir is non-writable")
		}
		binDir := filepath.Join(t.TempDir(), "bin")
		require.NoError(t, os.Mkdir(binDir, 0o555)) //nolint:gosec // intentionally non-writable
		runner := &fakeRunner{paths: map[string]bool{cmdSudo: true}}
		di := &defaultImplementation{runner: runner}
		opts := &GetOptions{BinDir: binDir}
		opts.Listener = &NoopListener{}

		src := writeSource(t)
		require.NoError(t, di.InstallAsset(opts, info, artifact, src))
		require.Equal(t, [][]string{
			{cmdSudo, verbInstall, "-m", "0755", src, filepath.Join(binDir, testAppName)},
		}, runner.run)
	})

	t.Run("non-writable-dir-no-sudo", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == system.OSWindows {
			t.Skip("directory permissions are not enforced on windows")
		}
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
	runner := &fakeRunner{paths: map[string]bool{cmdDnf: true, cmdSudo: true}}
	di := &defaultImplementation{runner: runner}
	opts := &GetOptions{}
	opts.Listener = &NoopListener{}
	artifact := &InstallArtifact{
		Kind: ArtifactPackage, PackageFormat: system.PackageRPM, InstallName: testAppName,
	}

	require.NoError(t, di.InstallAsset(opts, &system.Info{Os: system.OSLinux}, artifact, testRPMPath))
	require.Len(t, runner.run, 1)
	if os.Geteuid() == 0 {
		require.Equal(t, []string{cmdDnf, verbInstall, "-y", testRPMPath}, runner.run[0])
	} else {
		require.Equal(t, []string{cmdSudo, cmdDnf, verbInstall, "-y", testRPMPath}, runner.run[0])
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
		{name: "installed", format: system.PackageRPM, expect: true},
		{name: "not-installed", format: system.PackageRPM, silentErr: errors.New("exit 1"), expect: false},
		{name: "no-format", format: "", expect: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runner := &fakeRunner{
				paths:     map[string]bool{system.PackageRPM: true},
				silentErr: tc.silentErr,
			}
			di := &defaultImplementation{runner: runner}
			require.Equal(t, tc.expect, di.packageInstalled(tc.format, testAppName))
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
	opts.computedFilename = testRPMFile
	opts.Listener = &NoopListener{}
	asset := &github.Asset{
		Name:        testRPMFile,
		DownloadURL: srv.URL + "/drop-1.0.0-1.x86_64.rpm",
	}

	path, err := di.DownloadAssetToTmp(opts, asset)
	require.NoError(t, err)
	defer os.RemoveAll(filepath.Dir(path)) //nolint:errcheck

	require.Equal(t, testRPMFile, filepath.Base(path), "tmp file must keep the package extension")
	data, err := os.ReadFile(path) //nolint:gosec // path is a test-controlled tmp file
	require.NoError(t, err)
	require.Equal(t, "artifact-data", string(data))
}

func TestRecordInstall(t *testing.T) {
	t.Parallel()
	content := []byte("artifact-data")
	sum := sha256.Sum256(content)
	wantDigest := hex.EncodeToString(sum[:])

	asset := &github.Asset{
		Host:    "github.com",
		Org:     "carabiner-dev",
		Repo:    testAppName,
		Version: "v0.1.0",
		Name:    testBinFile,
		Os:      system.OSLinux,
		Arch:    system.ArchAMD64,
	}

	for _, tc := range []struct {
		name     string
		artifact *InstallArtifact
		verified bool
		check    func(t *testing.T, r *inventory.Record)
	}{
		{
			name: "binary",
			artifact: &InstallArtifact{
				Kind: ArtifactBinary, Asset: asset, InstallName: testAppName,
			},
			verified: true,
			check: func(t *testing.T, r *inventory.Record) {
				t.Helper()
				require.Equal(t, string(ArtifactBinary), r.Kind)
				require.Equal(t, filepath.Join("/opt/bin", testAppName), r.BinPath)
				require.Empty(t, r.PackageFormat)
				require.True(t, r.Verified)
			},
		},
		{
			name: "package-unverified",
			artifact: &InstallArtifact{
				Kind: ArtifactPackage, PackageFormat: system.PackageRPM,
				Asset: asset, InstallName: testAppName,
			},
			verified: false,
			check: func(t *testing.T, r *inventory.Record) {
				t.Helper()
				require.Equal(t, string(ArtifactPackage), r.Kind)
				require.Equal(t, system.PackageRPM, r.PackageFormat)
				require.Empty(t, r.BinPath)
				require.False(t, r.Verified)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			downloaded := filepath.Join(t.TempDir(), testBinFile)
			require.NoError(t, os.WriteFile(downloaded, content, 0o600))

			invPath := filepath.Join(t.TempDir(), "installed.json")
			di := &defaultImplementation{inventoryPath: invPath}
			opts := &GetOptions{BinDir: "/opt/bin"}

			require.NoError(t, di.RecordInstall(opts, tc.artifact, downloaded, tc.verified))

			inv, err := inventory.OpenFile(invPath)
			require.NoError(t, err)
			record := inv.Get("github.com/carabiner-dev/drop#drop")
			require.NotNil(t, record)
			require.Equal(t, testAppName, record.Name)
			require.Equal(t, "v0.1.0", record.Version)
			require.Equal(t, testBinFile, record.Asset)
			require.Equal(t, map[string]string{"sha256": wantDigest}, record.Digest)
			require.False(t, record.InstalledAt.IsZero())
			tc.check(t, record)
		})
	}
}
