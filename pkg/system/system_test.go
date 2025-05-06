package system

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMainSplitPattern(t *testing.T) {
	s := MainSplitPattern()
	require.Equal(t, "(?i)(aarch64|armv7hl|freebsd|illumos|openbsd|ppc64el|ppc64le|riscv64|solaris|windows|darwin|netbsd|x86_64|32bit|64bit|amd64|arm64|armhf|armv7|linux|macos|ppc64|s390x|i386|i686|386|arm|osx|x64|x86)", s)
}

func TestParseOSReleaseForFamily(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name           string
		path           string
		expectedFamily string
	}{
		{"alpine", "testdata/alpine.osrelease.txt", OSFamilyAlpine},
		{"fedora", "testdata/fedora.osrelease.txt", OSFamilyFedora},
		{"ubi", "testdata/ubi.osrelease.txt", OSFamilyRHEL},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f, err := os.Open(tc.path)
			require.NoError(t, err)

			fam := parseOSReleaseForFamily(f)
			require.Equal(t, tc.expectedFamily, fam)
		})
	}
}
