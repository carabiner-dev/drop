// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestGroupedUsage(t *testing.T) {
	t.Parallel()
	cmd := &cobra.Command{Use: "fake", Run: func(*cobra.Command, []string) {}}
	var binDir string
	cmd.PersistentFlags().StringVar(&binDir, "bin-dir", "/usr/local/bin", "directory to install binaries into")
	defaultAttestOptions().AddFlags(cmd)

	usage := cmd.UsageString()

	flags := strings.Index(usage, "Flags:\n")
	attest := strings.Index(usage, "Attestation Flags:\n")
	signing := strings.Index(usage, "Signing Flags:\n")
	require.NotEqual(t, -1, flags)
	require.NotEqual(t, -1, attest)
	require.NotEqual(t, -1, signing)
	require.Less(t, flags, attest, "the command's own flags come first")
	require.Less(t, attest, signing)

	own := usage[flags:attest]
	attestation := usage[attest:signing]
	sign := usage[signing:]
	require.Contains(t, own, "--bin-dir")
	require.NotContains(t, own, "--attest")
	require.Contains(t, attestation, "--attest ")
	require.Contains(t, attestation, "--attestation-type")
	require.Contains(t, attestation, "--sign ")
	require.NotContains(t, attestation, "--signing-key")
	require.Contains(t, sign, "--signing-key")
	require.Contains(t, sign, "--sigstore-instance")
	require.Contains(t, sign, "--spiffe-socket")
	require.NotContains(t, usage, "--sigstore-oidc", "hidden flags stay hidden")
}

func TestGroupedUsageWithoutGroups(t *testing.T) {
	t.Parallel()
	cmd := &cobra.Command{Use: "plain", Run: func(*cobra.Command, []string) {}}
	var name string
	cmd.PersistentFlags().StringVar(&name, "name", "", "a name")
	require.Equal(t, cmd.LocalFlags().FlagUsages(), groupedFlagUsages(cmd))
}
