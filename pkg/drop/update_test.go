// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package drop

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	v100    = "v1.0.0"
	build22 = "build-22"
)

func TestVersionIsNewer(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		installed string
		latest    string
		expect    bool
	}{
		{name: "newer-patch", installed: v100, latest: "v1.0.1", expect: true},
		{name: "newer-major", installed: "v1.9.9", latest: "v2.0.0", expect: true},
		{name: "same", installed: v100, latest: v100, expect: false},
		{name: "older", installed: "v2.0.0", latest: v100, expect: false},
		{name: "no-v-prefix", installed: "1.0.0", latest: "v1.1.0", expect: true},
		{name: "prerelease-to-release", installed: "v1.0.0-pre9", latest: v100, expect: true},
		{name: "release-to-prerelease", installed: v100, latest: "v1.0.0-pre9", expect: false},
		{name: "non-semver-differ", installed: build22, latest: "build-23", expect: true},
		{name: "non-semver-same", installed: build22, latest: build22, expect: false},
		{name: "empty-latest", installed: v100, latest: "", expect: false},
		{name: "empty-installed", installed: "", latest: v100, expect: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.expect, versionIsNewer(tc.installed, tc.latest))
		})
	}
}
