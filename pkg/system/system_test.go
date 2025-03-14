package system

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMainSplitPattern(t *testing.T) {
	s := MainSplitPattern()
	require.Equal(t, s, "(?i)(aarch64|armv7hl|freebsd|ppc64el|ppc64le|riscv64|windows|darwin|x86_64|amd64|arm64|armhf|armv7|linux|s390x|386|arm)")
}
