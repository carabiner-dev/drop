// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package drop

import (
	"errors"
	"runtime"
	"strings"

	"github.com/carabiner-dev/drop/pkg/system"
)

var defaultOptions = Options{}

var defaultGetOptions = GetOptions{
	DownloadPath: ".",
	OS:           runtime.GOOS,
	Arch:         runtime.GOARCH,
}

type Options struct {
	PolicyRepository string
}

type GetOptions struct {
	Options
	DownloadPath string
	OS           string
	Arch         string
	// Filename to store the downloaded asset
	FileName string
}

type FuncOption func(*Dropper) error
type FuncGetOption func(*GetOptions) error

// Constructor funcs
func WithPolicyRepository(repoURL string) FuncOption {
	return func(d *Dropper) error {
		d.Options.PolicyRepository = repoURL
		return nil
	}
}

// GetOptions
func WithPlatform(slug string) FuncGetOption {
	return func(o *GetOptions) error {
		os, arch, _ := strings.Cut(slug, "/")
		if os = system.GetOS(os); os == "" {
			return errors.New("invalid OS in platform slug")
		}
		if arch = system.GetArch(arch); arch == "" {
			return errors.New("invalid arch in platform slug")
		}

		o.OS = os
		o.Arch = arch
		return nil
	}
}

func WithDownloadPath(path string) FuncGetOption {
	return func(o *GetOptions) error {
		o.DownloadPath = path
		return nil
	}
}
