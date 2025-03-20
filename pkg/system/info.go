// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package system

import (
	"runtime"
)

type Info struct {
	Os   string
	Arch string
}

// GetInfo returns information about the running system
func GetInfo() (*Info, error) {
	return &Info{
		Os:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}, nil
}
