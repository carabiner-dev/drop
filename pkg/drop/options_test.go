// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package drop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithPolicyRepository(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing")
	cwd, err := os.Getwd()
	require.NoError(t, err)

	// file locators always use forward slashes and start with a slash
	locator := func(p string) string {
		p = filepath.ToSlash(p)
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}
		return "file://" + p
	}

	for _, tc := range []struct {
		name      string
		spec      string
		expect    string
		expectErr bool
	}{
		{"empty", "", "", false},
		{"slug", "sigstore/cosign", "https://github.com/sigstore/cosign", false},
		{"github-url", "https://github.com/carabiner-dev/demo-policy-store", "https://github.com/carabiner-dev/demo-policy-store", false},
		{"file-url", "file://" + dir, locator(dir), false},
		{"absolute-path", dir, locator(dir), false},
		{"dot", ".", locator(cwd), false},
		{"dot-relative", filepath.Join(".", "testdata", ".."), locator(cwd), false},
		{"missing-absolute", missing, "", true},
		{"missing-file-url", "file://" + missing, "", true},
		{"file-not-dir", "", "", true},
		{"bad-slug", "nothing", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			spec := tc.spec
			if tc.name == "file-not-dir" {
				f := filepath.Join(t.TempDir(), "policies.json")
				require.NoError(t, os.WriteFile(f, []byte("{}"), 0o600))
				spec = f
			}
			d := &Dropper{}
			err := WithPolicyRepository(spec)(d)
			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expect, d.Options.PolicyRepository)
		})
	}
}
