// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"errors"
	"fmt"
	"slices"

	"github.com/carabiner-dev/drop/internal/notifier"
	"github.com/carabiner-dev/drop/pkg/drop"
	"github.com/carabiner-dev/drop/pkg/github"
	"github.com/carabiner-dev/drop/pkg/system"
	"github.com/spf13/cobra"
)

type getOptions struct {
	AppUrl       string
	Platform     string
	PolicyRepo   string
	DownloadType string
	Timeout      int
	Quiet        bool
	Insecure     bool
}

var downloadTypes = []string{"binary", "package", "archive"}

// Validates the options in context with arguments
func (io *getOptions) Validate() error {
	errs := []error{}
	if io.AppUrl == "" {
		errs = append(errs, errors.New("app url not set"))
	}

	if io.Timeout == 0 {
		errs = append(errs, errors.New("timeout must be larger than zero"))
	}

	if io.DownloadType != "" && !slices.Contains(downloadTypes, io.DownloadType) &&
		io.DownloadType != "a" && io.DownloadType != "b" && io.DownloadType != "p" {
		errs = append(errs, fmt.Errorf("invalid download type valid types are %v", downloadTypes))
	}

	return errors.Join(errs...)
}

// AddFlags adds the subcommands flags
func (io *getOptions) AddFlags(cmd *cobra.Command) {
	platform := ""
	info, err := system.GetInfo()
	if err == nil {
		platform = fmt.Sprintf("%s/%s", info.Os, info.Arch)
	}
	cmd.PersistentFlags().StringVarP(
		&io.AppUrl, "app", "a", "", "app to install",
	)

	cmd.PersistentFlags().StringVarP(
		&io.Platform, "platform", "p", platform, "platform slug to download and verify",
	)

	cmd.PersistentFlags().StringVar(
		&io.PolicyRepo, "policy-repo", "", "alternative repository to use as policy source",
	)

	cmd.PersistentFlags().IntVar(
		&io.Timeout, "timeout", 900, "timeout (in seconds) to timeout downloads",
	)

	cmd.PersistentFlags().BoolVarP(
		&io.Quiet, "quiet", "q", false, "less verbose output (for scripts, etc)",
	)

	cmd.PersistentFlags().BoolVar(
		&io.Insecure, "insecure", false, "skip security verification (not recommended)",
	)

	cmd.PersistentFlags().StringVarP(
		&io.DownloadType, "type", "t", "", fmt.Sprintf("asset type to download (%v)", downloadTypes),
	)
}

func addGet(parentCmd *cobra.Command) {
	opts := &getOptions{}
	attCmd := &cobra.Command{
		Short: "downloads and verifies artifacts from GitHub releases",
		Long: fmt.Sprintf(`
%s

The %s subcommand downloads assets from a GitHub release. It is intended to
download installable artifacts but it can download, and potentially verify, any
file published as a release asset.

%s

By default, %s looks for attestations published along the artifacts and 
security policies in a specially named .ampel directory in the same GitHub
organization where the files are hosted. You can specify an alternative 
policy repository.

Artifacts in a release are grouped into an "installable". This is a named entry
that groups together all platform variants, packages and archives as well as 
their security metadata files (SBOMs, attestations, etc). The %s subcommand
lets you list the release installables and their components.

When downloading an installable, %s will download an installable that
matches the repository name and the version matching the local os+arch platform.
For example, on windows this invocation:

  drop get github.com/org/repo
  
will download a vinary to repo.exe for the local architecture. You can override
the variant to download with --platform:

  drop get --platform=linux/amd64 github.com/org/repo

If the installable does not match the repo name or the release has more than
one installable, you can specify another adding a frament (data after #) to the
app URL. For example, if "repo" publishes a "server" binary, you can dowload it
with:

  drop get github.com/org/repo#server


%s

All downloads are verified. If you really *really* want to skip the verification
process, you can add the --insecure flag:

  drop get --insecure github.com/shadyorg/repo

We would of course recommend that you suggest to the organization adding a couple
of %s policies to secure their releases ✨

`, DropBanner("Download and verify artifacts from GitHub releases"), w2("get"), w("SPECIFYING A DOWNLOAD"), w2("drop get"), w2("ls"), w2("drop get"), w("⚠️ Skipping Verification"), AmpelBanner(""),
		),
		Use:               "get",
		Example:           fmt.Sprintf(`%s get github.com/app/repo`, appname),
		SilenceUsage:      false,
		SilenceErrors:     true,
		PersistentPreRunE: initLogging,
		PreRunE: func(_ *cobra.Command, args []string) error {
			if len(args) > 0 && opts.AppUrl == "" {
				opts.AppUrl = args[0]
			}

			if len(args) > 0 && args[0] != opts.AppUrl {
				return fmt.Errorf("spec specified twice (--app and argument)")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			// Validate the options
			if err := opts.Validate(); err != nil {
				return err
			}
			cmd.SilenceUsage = true

			// Parse the asset URL
			asset := github.NewAssetFromURLString(opts.AppUrl)
			if asset == nil {
				return fmt.Errorf("unable to parse app URL: %q", opts.AppUrl)
			}

			if asset.Host == "" {
				asset.Host = "github.com"
			}

			// Set the CLI notifier as the notifier, unless -q was specified
			var lstnr drop.ProgressListener = &notifier.Listener{}
			if opts.Quiet {
				lstnr = &drop.NoopListener{}
			}

			// Create the new dropper instance
			dropper, err := drop.New(
				drop.WithPolicyRepository(opts.PolicyRepo),
				drop.WithListener(lstnr),
			)
			if err != nil {
				return fmt.Errorf("cerating dropper: %w", err)
			}

			// Run the download:
			if err := dropper.Get(
				asset,
				drop.WithDownloadPath("."),
				drop.WithTransferTimeOut(opts.Timeout),
				drop.WithPlatform(opts.Platform),
				drop.WithVerifyDownloads(!opts.Insecure),
				drop.WithDownloadType(opts.DownloadType),
			); err != nil {
				return fmt.Errorf("error downloading: %w", err)
			}
			return nil
		},
	}
	opts.AddFlags(attCmd)
	parentCmd.AddCommand(attCmd)
}
