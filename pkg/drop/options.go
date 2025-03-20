// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package drop

import (
	"errors"
	"fmt"
	"runtime"
	"strings"

	"github.com/carabiner-dev/drop/pkg/github"
	"github.com/carabiner-dev/drop/pkg/system"
)

var defaultOptions = Options{}

var defaultGetOptions = GetOptions{
	DownloadPath:    ".",
	OS:              runtime.GOOS,
	Arch:            runtime.GOARCH,
	TransferTimeOut: 900,
}

type Options struct {
	PolicyRepository string
	Listener         ProgressListener
}

type GetOptions struct {
	// Embedded dropper options to pass to implementation
	Options

	// Directory where the asset will be downloaded
	DownloadPath string

	// Platform to download
	OS   string
	Arch string

	// Filename to store the downloaded asset
	FileName string

	// computedFilename is the filename automatically determined when choosing
	// which file to download.
	computedFilename string

	// TransferTimeOut is the number of seconds after which the http request
	// will time out.
	TransferTimeOut int

	// SkipVerification instructs the dropper engine to skip the artifact
	// security verification. This allows the tool to be used as a curl-like
	// thing for repos.
	SkipVerification bool
}

type (
	FuncOption    func(*Dropper) error
	FuncGetOption func(*GetOptions) error
)

// Constructor funcs
func WithPolicyRepository(repoURL string) FuncOption {
	return func(d *Dropper) error {
		if repoURL == "" {
			d.Options.PolicyRepository = ""
			return nil
		}
		str, err := github.RepoURLFromString(repoURL)
		if err != nil {
			return err
		}
		d.Options.PolicyRepository = str
		return nil
	}
}

func WithListener(listener ProgressListener) FuncOption {
	return func(d *Dropper) error {
		d.Options.Listener = listener
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

func WithTransferTimeOut(seconds int) FuncGetOption {
	return func(o *GetOptions) error {
		if seconds == 0 {
			return fmt.Errorf("transfer timeout seconds cannot be zer")
		}
		o.TransferTimeOut = seconds
		return nil
	}
}

func WithVerifyDownloads(verify bool) FuncGetOption {
	return func(o *GetOptions) error {
		o.SkipVerification = !verify
		return nil
	}
}
