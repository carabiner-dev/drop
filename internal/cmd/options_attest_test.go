// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/carabiner-dev/drop/pkg/drop"
)

func TestAttestOptionsValidate(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		configure func(ao *attestOptions)
		insecure  bool
		expectErr string
	}{
		{name: "defaults"},
		{name: "attest-with-defaults", configure: func(ao *attestOptions) { ao.Attest = true }},
		{
			name:      "out-without-attest",
			configure: func(ao *attestOptions) { ao.Out = "evidence/" },
			expectErr: "--attestation-out requires --attest",
		},
		{
			name:      "unsigned-without-attest",
			configure: func(ao *attestOptions) { ao.Sign = false },
			expectErr: "--sign=false requires --attest",
		},
		{
			name:      "attest-with-insecure",
			configure: func(ao *attestOptions) { ao.Attest = true },
			insecure:  true,
			expectErr: "cannot be combined with --insecure",
		},
		{
			name:      "bad-type",
			configure: func(ao *attestOptions) { ao.Attest, ao.Type = true, "yaml" },
			expectErr: `invalid attestation type "yaml"`,
		},
		{
			name: "every-type",
			configure: func(ao *attestOptions) {
				ao.Attest, ao.Type = true, "svr"
			},
		},
		{
			name: "missing-signing-key",
			configure: func(ao *attestOptions) {
				ao.Attest = true
				ao.Keys.PrivateKeyPaths = []string{filepath.Join(t.TempDir(), "nope.pem")}
			},
			expectErr: "signing options",
		},
		{
			name: "unsigned-skips-signer-validation",
			configure: func(ao *attestOptions) {
				ao.Attest, ao.Sign = true, false
				ao.Keys.PrivateKeyPaths = []string{filepath.Join(t.TempDir(), "nope.pem")}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ao := defaultAttestOptions()
			if tc.configure != nil {
				tc.configure(ao)
			}
			err := ao.Validate(tc.insecure)
			if tc.expectErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.expectErr)
		})
	}
}

func TestAttestOptionsDropperOptions(t *testing.T) {
	t.Parallel()
	apply := func(t *testing.T, opts []drop.FuncOption) *drop.Dropper {
		t.Helper()
		d := &drop.Dropper{}
		for _, fn := range opts {
			require.NoError(t, fn(d))
		}
		return d
	}

	t.Run("off", func(t *testing.T) {
		t.Parallel()
		opts, closer, err := defaultAttestOptions().DropperOptions(context.Background())
		require.NoError(t, err)
		require.Empty(t, opts)
		closer()
	})

	t.Run("unsigned", func(t *testing.T) {
		t.Parallel()
		ao := defaultAttestOptions()
		ao.Attest, ao.Sign, ao.Type = true, false, "vsa"
		opts, closer, err := ao.DropperOptions(context.Background())
		require.NoError(t, err)
		defer closer()
		d := apply(t, opts)
		require.True(t, d.Options.Attest)
		require.Equal(t, "vsa", d.Options.AttestFormat)
		require.Nil(t, d.Options.Signer, "no signer is built when not signing")
	})

	t.Run("unreadable-key", func(t *testing.T) {
		t.Parallel()
		ao := defaultAttestOptions()
		ao.Attest = true
		ao.Keys.PrivateKeyPaths = []string{filepath.Join(t.TempDir(), "nope.pem")}
		_, _, err := ao.DropperOptions(context.Background())
		require.ErrorContains(t, err, "building signer")
	})
}
