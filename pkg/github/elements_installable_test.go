package github

import (
	"fmt"
	"testing"

	"github.com/carabiner-dev/drop/pkg/system"
	"github.com/stretchr/testify/require"
)

func TestGetArchFromFilename(t *testing.T) {
	expect := []string{
		system.ArchArm64, system.ArchArm64, system.ArchArm64, system.ArchArm, system.ArchArm,
		system.ArchArm, system.ArchPPC64LE, system.ArchPPC64LE, system.ArchPPC64LE, system.ArchRiscV64,
		system.ArchRiscV64, system.ArchRiscV64, system.ArchS390X, system.ArchS390X, system.ArchS390X,
		system.ArchX8664, system.ArchX8664, system.ArchX8664, system.ArchX8664, system.ArchX8664,
		system.ArchX8664, system.ArchX8664, system.ArchX8664, system.ArchArm64, system.ArchArm64,
		system.ArchArm64, system.ArchArm64, system.ArchArm64, system.ArchX8664, system.ArchX8664,
		system.ArchX8664, system.ArchX8664, system.ArchX8664, system.ArchArm, system.ArchArm,
		system.ArchArm, system.ArchArm, system.ArchArm64, system.ArchArm64, system.ArchArm64,
		system.ArchArm64, system.ArchArm64, system.ArchArm, system.ArchX8664, system.ArchX8664,
		system.ArchX8664, system.ArchX8664, system.ArchX8664, system.ArchArm64, system.ArchArm64,
		system.ArchArm64, system.ArchArm64, system.ArchArm64, system.ArchPPC64LE, system.ArchPPC64LE,
		system.ArchPPC64LE, system.ArchPPC64LE, system.ArchPPC64LE, system.ArchRiscV64, system.ArchRiscV64,
		system.ArchRiscV64, system.ArchRiscV64, system.ArchRiscV64, system.ArchS390X, system.ArchS390X,
		system.ArchS390X, system.ArchS390X, system.ArchS390X, system.ArchX8664, system.ArchX8664,
		system.ArchX8664, system.ArchX8664, system.ArchX8664, system.ArchArm64, system.ArchArm64,
		system.ArchArm64, system.ArchX8664, system.ArchX8664, system.ArchX8664, system.ArchArm64,
		system.ArchArm64, system.ArchArm64, system.ArchArm, system.ArchArm, system.ArchArm,
		system.ArchArm, system.ArchArm, system.ArchArm, system.ArchPPC64LE, system.ArchPPC64LE,
		system.ArchPPC64LE, system.ArchPPC64LE, system.ArchPPC64LE, system.ArchPPC64LE, system.ArchRiscV64,
		system.ArchRiscV64, system.ArchRiscV64, system.ArchRiscV64, system.ArchRiscV64, system.ArchRiscV64,
		system.ArchS390X, system.ArchS390X, system.ArchS390X, system.ArchS390X, system.ArchS390X,
		system.ArchS390X, system.ArchX8664, system.ArchX8664, system.ArchX8664, "", "", "", "",
	}
	// expect := make([]string, len(fileSet2))
	for i, filename := range fileSet2 {
		t.Run(filename, func(t *testing.T) {
			t.Parallel()
			res := getArchFromFilename(filename)
			require.Equal(t, expect[i], res, fmt.Sprintf("%d → %s", i, filename))
		})
	}
}

func TestGetOsFromFilename(t *testing.T) {
	expect := []string{
		system.OSDarwin, system.OSLinux, system.OSWindows,
		system.OSLinux, system.OSDarwin, system.OSLinux,
		system.OSLinux, system.OSLinux, "",
		"", "", "", system.OSDarwin,
		system.OSLinux, system.OSWindows, system.OSLinux, system.OSDarwin,
		system.OSLinux, system.OSLinux, system.OSLinux, "",
		system.OSDarwin, system.OSLinux, system.OSWindows,
		system.OSLinux, system.OSDarwin, system.OSLinux,
		system.OSLinux, system.OSLinux, "",
	}
	// expect := make([]string, len(fileSet2))
	for i, filename := range fileSet1 {
		t.Run(filename, func(t *testing.T) {
			t.Parallel()
			res := getOsFromFilename(filename)
			require.Equal(t, expect[i], res, fmt.Sprintf("%d → %s", i, filename))
		})
	}
}

func TestTrimSeparatorSuffix(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		sut      string
		expected string
	}{
		{"nochange", "binary", "binary"},
		{"dot", "binary.", "binary"},
		{"underscore", "binary_", "binary"},
		{"dash", "binary-", "binary"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res := trimSeparatorSuffix(tc.sut)
			require.Equal(t, tc.expected, res)
		})
	}
}

var fileSet1 = []string{
	"bom-amd64-darwin.sig", "bom-amd64-linux.sig", "bom-amd64-windows.exe.sig",
	"bom-arm-linux.sig", "bom-arm64-darwin.sig", "bom-arm64-linux.sig",
	"bom-ppc64le-linux.sig", "bom-s390x-linux.sig", "bom.json.spdx.sig",
	"checksums.txt", "checksums.txt.pem", "checksums.txt.sig", "bom-amd64-darwin",
	"bom-amd64-linux", "bom-amd64-windows.exe", "bom-arm-linux", "bom-arm64-darwin",
	"bom-arm64-linux", "bom-ppc64le-linux", "bom-s390x-linux", "bom.json.spdx",
	"bom-amd64-darwin.pem", "bom-amd64-linux.pem", "bom-amd64-windows.exe.pem",
	"bom-arm-linux.pem", "bom-arm64-darwin.pem", "bom-arm64-linux.pem",
	"bom-ppc64le-linux.pem", "bom-s390x-linux.pem", "bom.json.spdx.pem",
}
var fileSet2 = []string{
	"cosign-2.4.3-1.aarch64.rpm", "cosign-2.4.3-1.aarch64.rpm-keyless.pem",
	"cosign-2.4.3-1.aarch64.rpm-keyless.sig", "cosign-2.4.3-1.armv7hl.rpm",
	"cosign-2.4.3-1.armv7hl.rpm-keyless.pem", "cosign-2.4.3-1.armv7hl.rpm-keyless.sig",
	"cosign-2.4.3-1.ppc64le.rpm", "cosign-2.4.3-1.ppc64le.rpm-keyless.pem",
	"cosign-2.4.3-1.ppc64le.rpm-keyless.sig", "cosign-2.4.3-1.riscv64.rpm",
	"cosign-2.4.3-1.riscv64.rpm-keyless.pem", "cosign-2.4.3-1.riscv64.rpm-keyless.sig",
	"cosign-2.4.3-1.s390x.rpm", "cosign-2.4.3-1.s390x.rpm-keyless.pem",
	"cosign-2.4.3-1.s390x.rpm-keyless.sig", "cosign-2.4.3-1.x86_64.rpm",
	"cosign-2.4.3-1.x86_64.rpm-keyless.pem", "cosign-2.4.3-1.x86_64.rpm-keyless.sig",
	"cosign-darwin-amd64",
	"cosign-darwin-amd64-keyless.pem",
	"cosign-darwin-amd64-keyless.sig",
	"cosign-darwin-amd64.sig",
	"cosign-darwin-amd64_2.4.3_darwin_amd64.sbom.json",
	"cosign-darwin-arm64",
	"cosign-darwin-arm64-keyless.pem",
	"cosign-darwin-arm64-keyless.sig",
	"cosign-darwin-arm64.sig",
	"cosign-darwin-arm64_2.4.3_darwin_arm64.sbom.json",
	"cosign-linux-amd64", "cosign-linux-amd64-keyless.pem",
	"cosign-linux-amd64-keyless.sig", "cosign-linux-amd64.sig",
	"cosign-linux-amd64_2.4.3_linux_amd64.sbom.json", "cosign-linux-arm",
	"cosign-linux-arm-keyless.pem", "cosign-linux-arm-keyless.sig",
	"cosign-linux-arm.sig", "cosign-linux-arm64", "cosign-linux-arm64-keyless.pem",
	"cosign-linux-arm64-keyless.sig", "cosign-linux-arm64.sig",
	"cosign-linux-arm64_2.4.3_linux_arm64.sbom.json",
	"cosign-linux-arm_2.4.3_linux_arm.sbom.json", "cosign-linux-pivkey-pkcs11key-amd64",
	"cosign-linux-pivkey-pkcs11key-amd64-keyless.pem",
	"cosign-linux-pivkey-pkcs11key-amd64-keyless.sig",
	"cosign-linux-pivkey-pkcs11key-amd64.sig",
	"cosign-linux-pivkey-pkcs11key-amd64_2.4.3_linux_amd64.sbom.json",
	"cosign-linux-pivkey-pkcs11key-arm64",
	"cosign-linux-pivkey-pkcs11key-arm64-keyless.pem",
	"cosign-linux-pivkey-pkcs11key-arm64-keyless.sig",
	"cosign-linux-pivkey-pkcs11key-arm64.sig",
	"cosign-linux-pivkey-pkcs11key-arm64_2.4.3_linux_arm64.sbom.json",
	"cosign-linux-ppc64le",
	"cosign-linux-ppc64le-keyless.pem",
	"cosign-linux-ppc64le-keyless.sig",
	"cosign-linux-ppc64le.sig",
	"cosign-linux-ppc64le_2.4.3_linux_ppc64le.sbom.json",
	"cosign-linux-riscv64",
	"cosign-linux-riscv64-keyless.pem",
	"cosign-linux-riscv64-keyless.sig",
	"cosign-linux-riscv64.sig",
	"cosign-linux-riscv64_2.4.3_linux_riscv64.sbom.json",
	"cosign-linux-s390x",
	"cosign-linux-s390x-keyless.pem",
	"cosign-linux-s390x-keyless.sig",
	"cosign-linux-s390x.sig",
	"cosign-linux-s390x_2.4.3_linux_s390x.sbom.json",
	"cosign-windows-amd64.exe",
	"cosign-windows-amd64.exe-keyless.pem",
	"cosign-windows-amd64.exe-keyless.sig",
	"cosign-windows-amd64.exe.sig",
	"cosign-windows-amd64.exe_2.4.3_windows_amd64.sbom.json",
	"cosign_2.4.3_aarch64.apk",
	"cosign_2.4.3_aarch64.apk-keyless.pem",
	"cosign_2.4.3_aarch64.apk-keyless.sig",
	"cosign_2.4.3_amd64.deb",
	"cosign_2.4.3_amd64.deb-keyless.pem",
	"cosign_2.4.3_amd64.deb-keyless.sig",
	"cosign_2.4.3_arm64.deb",
	"cosign_2.4.3_arm64.deb-keyless.pem", "cosign_2.4.3_arm64.deb-keyless.sig",
	"cosign_2.4.3_armhf.deb",
	"cosign_2.4.3_armhf.deb-keyless.pem",
	"cosign_2.4.3_armhf.deb-keyless.sig",
	"cosign_2.4.3_armv7.apk",
	"cosign_2.4.3_armv7.apk-keyless.pem", "cosign_2.4.3_armv7.apk-keyless.sig",
	"cosign_2.4.3_ppc64el.deb", "cosign_2.4.3_ppc64el.deb-keyless.pem",
	"cosign_2.4.3_ppc64el.deb-keyless.sig", "cosign_2.4.3_ppc64le.apk",
	"cosign_2.4.3_ppc64le.apk-keyless.pem", "cosign_2.4.3_ppc64le.apk-keyless.sig",
	"cosign_2.4.3_riscv64.apk", "cosign_2.4.3_riscv64.apk-keyless.pem",
	"cosign_2.4.3_riscv64.apk-keyless.sig", "cosign_2.4.3_riscv64.deb",
	"cosign_2.4.3_riscv64.deb-keyless.pem", "cosign_2.4.3_riscv64.deb-keyless.sig",
	"cosign_2.4.3_s390x.apk", "cosign_2.4.3_s390x.apk-keyless.pem",
	"cosign_2.4.3_s390x.apk-keyless.sig", "cosign_2.4.3_s390x.deb",
	"cosign_2.4.3_s390x.deb-keyless.pem", "cosign_2.4.3_s390x.deb-keyless.sig",
	"cosign_2.4.3_x86_64.apk", "cosign_2.4.3_x86_64.apk-keyless.pem",
	"cosign_2.4.3_x86_64.apk-keyless.sig", "cosign_checksums.txt",
	"cosign_checksums.txt-keyless.pem", "cosign_checksums.txt-keyless.sig",
	"release-cosign.pub",
}

func TestFileListToInstallableList(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
	}{} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
		})
	}
}
