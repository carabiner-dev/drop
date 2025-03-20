package system

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMainSplitPattern(t *testing.T) {
	s := MainSplitPattern()
	require.Equal(t, "(?i)(aarch64|armv7hl|freebsd|illumos|openbsd|ppc64el|ppc64le|riscv64|solaris|windows|darwin|netbsd|x86_64|32bit|64bit|amd64|arm64|armhf|armv7|linux|macos|ppc64|s390x|i386|i686|386|arm|osx|x64|x86)", s)
}
