// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package drop

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/carabiner-dev/drop/pkg/github"
	"github.com/carabiner-dev/drop/pkg/system"
)

var (
	ErrNoInstallableArtifact = errors.New("release has no binary or compatible package for this platform")
	ErrOnlyArchives          = errors.New("release only ships archives for this platform")
)

// ArtifactKind distinguishes the kinds of artifacts the installer can handle.
type ArtifactKind string

const (
	ArtifactBinary  ArtifactKind = "binary"
	ArtifactPackage ArtifactKind = "package"
)

// Command and filename constants used when installing artifacts
const (
	cmdSudo     = "sudo"
	cmdDnf      = "dnf"
	cmdYum      = "yum"
	cmdRPM      = "rpm"
	cmdApt      = "apt"
	cmdDpkg     = "dpkg"
	cmdApk      = "apk"
	verbInstall = "install"
	exeSuffix   = ".exe"

	dataKeyKind = "kind"
	dataKeyName = "name"
	dataKeySudo = "sudo"
)

// InstallArtifact is a concrete release asset chosen for installation.
type InstallArtifact struct {
	// Kind is the artifact type, binary or package.
	Kind ArtifactKind

	// PackageFormat is the package type (rpm, deb, apk) when Kind is package.
	PackageFormat string

	// Asset is the release asset variant to download.
	Asset *github.Asset

	// InstallName is the name the binary gets when installed into the path.
	InstallName string
}

// ArtifactSelector resolves an ambiguous choice between install candidates.
// The CLI injects an interactive implementation when running on a terminal.
type ArtifactSelector func(candidates []*InstallArtifact) (*InstallArtifact, error)

// installCandidates is the classified view of an installable's variants
// for a single platform.
type installCandidates struct {
	Binary      *InstallArtifact
	Package     *InstallArtifact
	HasArchives bool
	HasOtherPkg bool
}

// classifyInstallCandidates inspects an installable's variants for the given
// platform and classifies them into a binary candidate and a package candidate
// matching the system's package format.
func classifyInstallCandidates(inst *github.Installable, osName, arch, pkgFormat string) *installCandidates {
	cands := &installCandidates{}
	for _, variant := range inst.Variants {
		if variant.Os != osName || variant.Arch != arch {
			continue
		}

		packageType := system.PackageExtensions.GetTypeFromFile(variant.GetName())
		archiveType := system.ArchiveExtensions.GetTypeFromFile(variant.GetName())

		switch {
		case archiveType != "":
			cands.HasArchives = true
		case packageType == "":
			name := inst.GetName()
			if variant.Os == system.OSWindows {
				name += exeSuffix
			}
			cands.Binary = &InstallArtifact{
				Kind: ArtifactBinary, Asset: variant, InstallName: name,
			}
		case pkgFormat != "" && packageType == pkgFormat:
			cands.Package = &InstallArtifact{
				Kind: ArtifactPackage, PackageFormat: packageType,
				Asset: variant, InstallName: inst.GetName(),
			}
		default:
			cands.HasOtherPkg = true
		}
	}
	return cands
}

// classifySingleAsset builds an install artifact from a single concrete asset,
// for when the user pinned an exact file instead of an installable.
func classifySingleAsset(asset *github.Asset, installName, pkgFormat string) (*InstallArtifact, error) {
	name := asset.GetName()
	if system.ArchiveExtensions.GetTypeFromFile(name) != "" {
		return nil, ErrOnlyArchives
	}
	if pkgType := system.PackageExtensions.GetTypeFromFile(name); pkgType != "" {
		if pkgFormat == "" || pkgType != pkgFormat {
			return nil, ErrNoInstallableArtifact
		}
		return &InstallArtifact{
			Kind: ArtifactPackage, PackageFormat: pkgType,
			Asset: asset, InstallName: installName,
		}, nil
	}
	if asset.Os == system.OSWindows && !strings.HasSuffix(installName, exeSuffix) {
		installName += exeSuffix
	}
	return &InstallArtifact{
		Kind: ArtifactBinary, Asset: asset, InstallName: installName,
	}, nil
}

// decideArtifact applies the install selection algorithm: honor a forced type,
// use the only candidate available, stay with the package manager when the app
// is already installed as a package, otherwise ask the selector (or default to
// the binary when running non-interactively).
func decideArtifact(c *installCandidates, opts *GetOptions, pkgInstalled func(name string) bool) (*InstallArtifact, error) {
	switch opts.DownloadType {
	case "b":
		if c.Binary == nil {
			return nil, fmt.Errorf("no binary available: %w", ErrNoInstallableArtifact)
		}
		return c.Binary, nil
	case "p":
		if c.Package == nil {
			return nil, fmt.Errorf("no package in the system format available: %w", ErrNoInstallableArtifact)
		}
		return c.Package, nil
	}

	switch {
	case c.Binary == nil && c.Package == nil:
		if c.HasArchives {
			return nil, ErrOnlyArchives
		}
		return nil, ErrNoInstallableArtifact
	case c.Package == nil:
		return c.Binary, nil
	case c.Binary == nil:
		return c.Package, nil
	}

	if pkgInstalled != nil && pkgInstalled(c.Package.InstallName) {
		return c.Package, nil
	}

	if opts.Selector != nil {
		return opts.Selector([]*InstallArtifact{c.Binary, c.Package})
	}

	return c.Binary, nil
}

// buildPackageInstallCmd returns the argv to install a local package file
// using the system's package manager.
func buildPackageInstallCmd(format, pkgPath string, sudo bool, lookPath func(string) (string, error)) ([]string, error) {
	has := func(tool string) bool {
		_, err := lookPath(tool)
		return err == nil
	}

	var argv []string
	switch format {
	case system.PackageRPM:
		switch {
		case has(cmdDnf):
			argv = []string{cmdDnf, verbInstall, "-y", pkgPath}
		case has(cmdYum):
			argv = []string{cmdYum, verbInstall, "-y", pkgPath}
		case has(cmdRPM):
			argv = []string{cmdRPM, "-Uvh", pkgPath}
		default:
			return nil, errors.New("no rpm package manager (dnf/yum/rpm) found in PATH")
		}
	case system.PackageDeb:
		// apt needs a path (not a package name) to install a local file
		abs, err := filepath.Abs(pkgPath)
		if err != nil {
			return nil, fmt.Errorf("resolving package path: %w", err)
		}
		switch {
		case has(cmdApt):
			argv = []string{cmdApt, verbInstall, "-y", abs}
		case has(cmdDpkg):
			argv = []string{cmdDpkg, "-i", abs}
		default:
			return nil, errors.New("no deb package manager (apt/dpkg) found in PATH")
		}
	case system.PackageApk:
		if !has(cmdApk) {
			return nil, errors.New("apk not found in PATH")
		}
		// Local apk files are not signed by a repository key, the artifact
		// was already verified against its policies before reaching this.
		argv = []string{cmdApk, "add", "--allow-untrusted", pkgPath}
	default:
		return nil, fmt.Errorf("unsupported package format %q", format)
	}

	if sudo {
		if !has(cmdSudo) {
			return nil, errors.New("sudo not found in PATH, rerun as root")
		}
		argv = append([]string{cmdSudo}, argv...)
	}
	return argv, nil
}

// buildPackageQueryCmd returns the argv that checks if a package is installed
// in the system. A zero exit code means the package is present.
func buildPackageQueryCmd(format, name string, lookPath func(string) (string, error)) ([]string, error) {
	has := func(tool string) bool {
		_, err := lookPath(tool)
		return err == nil
	}

	switch format {
	case system.PackageRPM:
		if !has(cmdRPM) {
			return nil, errors.New("rpm not found in PATH")
		}
		return []string{cmdRPM, "-q", name}, nil
	case system.PackageDeb:
		if !has(cmdDpkg) {
			return nil, errors.New("dpkg not found in PATH")
		}
		return []string{cmdDpkg, "-s", name}, nil
	case system.PackageApk:
		if !has(cmdApk) {
			return nil, errors.New("apk not found in PATH")
		}
		return []string{cmdApk, "info", "-e", name}, nil
	default:
		return nil, fmt.Errorf("unsupported package format %q", format)
	}
}

// commandRunner abstracts running external commands so the install logic
// can be tested without touching the system.
type commandRunner interface {
	// Run executes a command inheriting the standard streams (so tools
	// like sudo can prompt the user).
	Run(argv []string) error

	// RunSilent executes a command discarding its output.
	RunSilent(argv []string) error

	// LookPath checks if an executable is available in the system path.
	LookPath(file string) (string, error)
}

type execRunner struct{}

func (*execRunner) Run(argv []string) error {
	cmd := exec.CommandContext(context.Background(), argv[0], argv[1:]...) //nolint:gosec // argv comes from fixed command tables
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (*execRunner) RunSilent(argv []string) error {
	return exec.CommandContext(context.Background(), argv[0], argv[1:]...).Run() //nolint:gosec // argv comes from fixed command tables
}

func (*execRunner) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

// dirWritable probes a directory for write access by creating a temp file.
func dirWritable(dir string) bool {
	f, err := os.CreateTemp(dir, ".drop-write-check-")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()       //nolint:errcheck
	_ = os.Remove(name) //nolint:errcheck
	return true
}

// copyFile copies src to dst setting the supplied mode.
func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src) //nolint:gosec
	if err != nil {
		return fmt.Errorf("opening source file: %w", err)
	}
	defer in.Close() //nolint:errcheck

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode) //nolint:gosec
	if err != nil {
		return fmt.Errorf("creating target file: %w", err)
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close() //nolint:errcheck
		return fmt.Errorf("copying file data: %w", err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("closing target file: %w", err)
	}
	return os.Chmod(dst, mode)
}

// specName returns the name of the artifact a spec points to, defaulting to
// the repository name when the spec does not pin an asset name.
func specName(spec github.AssetDataProvider) string {
	if spec.GetName() != "" {
		return spec.GetName()
	}
	return spec.GetRepo()
}

// SelectInstallArtifact decides which release artifact (binary or system
// package) will be installed on the local system.
func (di *defaultImplementation) SelectInstallArtifact(
	opts *GetOptions, client *github.Client, info *system.Info, spec github.AssetDataProvider,
) (*InstallArtifact, error) {
	assets, err := client.ListReleaseInstallables(spec)
	if err != nil {
		return nil, fmt.Errorf("fetching release assets: %w", err)
	}

	// Compute the package format the system prefers. dmg and msi installs
	// are not supported yet, so macOS and Windows are binary-only for now.
	pkgFormat := system.GetPreferredPackage(info.Family)
	binaryOnly := info.Family == system.OSFamilyMacOS || info.Family == system.OSFamilyWindows
	if binaryOnly {
		pkgFormat = ""
	}

	found := findInstallable(assets, spec)
	if found == nil {
		// Check the variant filenames in case the user pinned an exact file
		// in the URL spec:
		name := specName(spec)
		for _, a := range assets {
			inst, ok := a.(*github.Installable)
			if !ok {
				continue
			}
			for _, v := range inst.Variants {
				if v.GetName() != name {
					continue
				}
				artifact, err := classifySingleAsset(v, inst.GetName(), pkgFormat)
				if err != nil {
					return nil, err
				}
				opts.computedFilename = v.GetName()
				return artifact, nil
			}
		}
		return nil, fmt.Errorf("no asset found for %s", spec.GetRepo())
	}

	inst, ok := found.(*github.Installable)
	if !ok {
		// A plain asset without platform variants, treat it as a single file
		plain, ok := found.(*github.Asset)
		if !ok {
			return nil, ErrNoInstallableArtifact
		}
		artifact, err := classifySingleAsset(plain, plain.GetName(), pkgFormat)
		if err != nil {
			return nil, err
		}
		opts.computedFilename = plain.GetName()
		return artifact, nil
	}

	cands := classifyInstallCandidates(inst, opts.OS, opts.Arch, pkgFormat)

	if binaryOnly && cands.HasOtherPkg {
		opts.Listener.HandleEvent(&Event{
			Object: EventObjectInstall, Verb: EventVerbSkipped,
			Data: map[string]string{"reason": "dmg/msi installation is not supported yet"},
		})
	}

	artifact, err := decideArtifact(cands, opts, func(name string) bool {
		return di.packageInstalled(pkgFormat, name)
	})
	if err != nil {
		return nil, err
	}

	opts.computedFilename = artifact.Asset.GetName()
	return artifact, nil
}

// packageInstalled checks (best effort) if a package is already installed in
// the system. The queried name is the installable name, which may differ from
// the actual package name; a miss only means the user gets asked.
func (di *defaultImplementation) packageInstalled(format, name string) bool {
	if format == "" || name == "" {
		return false
	}
	argv, err := buildPackageQueryCmd(format, name, di.runner.LookPath)
	if err != nil {
		return false
	}
	return di.runner.RunSilent(argv) == nil
}

// InstallAsset invokes the system mechanism to set up the downloaded artifact
// in the local machine.
func (di *defaultImplementation) InstallAsset(
	opts *GetOptions, info *system.Info, artifact *InstallArtifact, path string,
) error {
	switch artifact.Kind {
	case ArtifactBinary:
		return di.installBinary(opts, info, artifact, path)
	case ArtifactPackage:
		return di.installPackage(opts, artifact, path)
	default:
		return fmt.Errorf("unknown artifact kind %q", artifact.Kind)
	}
}

// installBinary copies the downloaded binary to the configured directory,
// shelling out to sudo when the directory is not writable by the user.
func (di *defaultImplementation) installBinary(
	opts *GetOptions, info *system.Info, artifact *InstallArtifact, path string,
) error {
	target := filepath.Join(opts.BinDir, artifact.InstallName)
	sudo := !dirWritable(opts.BinDir)

	if sudo {
		if info.Os == system.OSWindows {
			return fmt.Errorf("directory %q is not writable", opts.BinDir)
		}
		if _, err := di.runner.LookPath(cmdSudo); err != nil {
			return fmt.Errorf("%q is not writable and sudo is not available, rerun as root or set another binary directory", opts.BinDir)
		}
	}

	opts.Listener.HandleEvent(&Event{
		Object: EventObjectInstall, Verb: EventVerbRunning,
		Data: map[string]string{
			dataKeyKind: string(ArtifactBinary),
			dataKeyName: artifact.InstallName,
			"target":    target,
			dataKeySudo: strconv.FormatBool(sudo),
		},
	})

	if sudo {
		if err := di.runner.Run([]string{cmdSudo, verbInstall, "-m", "0755", path, target}); err != nil {
			return fmt.Errorf("installing binary: %w", err)
		}
	} else {
		if err := copyFile(path, target, 0o755); err != nil {
			return fmt.Errorf("installing binary: %w", err)
		}
	}

	opts.Listener.HandleEvent(&Event{
		Object: EventObjectInstall, Verb: EventVerbDone,
		Data: map[string]string{
			dataKeyKind: string(ArtifactBinary),
			dataKeyName: artifact.InstallName,
			"path":      target,
		},
	})
	return nil
}

// installPackage installs the downloaded package using the system's package
// manager, through sudo when not running as root.
func (di *defaultImplementation) installPackage(
	opts *GetOptions, artifact *InstallArtifact, path string,
) error {
	sudo := os.Geteuid() != 0
	argv, err := buildPackageInstallCmd(artifact.PackageFormat, path, sudo, di.runner.LookPath)
	if err != nil {
		return err
	}

	opts.Listener.HandleEvent(&Event{
		Object: EventObjectInstall, Verb: EventVerbRunning,
		Data: map[string]string{
			dataKeyKind: string(ArtifactPackage),
			"format":    artifact.PackageFormat,
			dataKeyName: artifact.InstallName,
			dataKeySudo: strconv.FormatBool(sudo),
		},
	})

	if err := di.runner.Run(argv); err != nil {
		return fmt.Errorf("installing %s package: %w", artifact.PackageFormat, err)
	}

	opts.Listener.HandleEvent(&Event{
		Object: EventObjectInstall, Verb: EventVerbDone,
		Data: map[string]string{
			dataKeyKind: string(ArtifactPackage),
			dataKeyName: artifact.InstallName,
		},
	})
	return nil
}
