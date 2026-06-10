// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/carabiner-dev/drop/internal/notifier"
	"github.com/carabiner-dev/drop/pkg/drop"
	"github.com/carabiner-dev/drop/pkg/github"
)

type installOptions struct {
	AppUrl      string
	PolicyRepo  string
	InstallType string
	Timeout     int
	Quiet       bool
	Insecure    bool
	BinDir      string
}

var installTypes = []string{string(drop.ArtifactBinary), string(drop.ArtifactPackage)}

// Validates the options in context with arguments
func (io *installOptions) Validate() error {
	errs := []error{}
	if io.AppUrl == "" {
		errs = append(errs, errors.New("app url not set"))
	}

	if io.Timeout == 0 {
		errs = append(errs, errors.New("timeout must be larger than zero"))
	}

	switch io.InstallType {
	case "", "b", "p", string(drop.ArtifactBinary), string(drop.ArtifactPackage):
	case "a", "archive":
		errs = append(errs, errors.New("archives cannot be installed, use \"drop get\" to download them"))
	default:
		errs = append(errs, fmt.Errorf("invalid install type, valid types are %v", installTypes))
	}

	if io.BinDir == "" {
		errs = append(errs, errors.New("binary directory cannot be empty"))
	}

	return errors.Join(errs...)
}

// AddFlags adds the subcommands flags
func (io *installOptions) AddFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(
		&io.AppUrl, "app", "a", "", "app to install",
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
		&io.InstallType, "type", "t", "", fmt.Sprintf("artifact type to install (%v)", installTypes),
	)

	cmd.PersistentFlags().StringVar(
		&io.BinDir, "bin-dir", "/usr/local/bin", "directory to install binaries into",
	)
}

func addInstall(parentCmd *cobra.Command) {
	opts := &installOptions{}
	attCmd := &cobra.Command{
		Short: "installs a binary or package after verifying it",
		Long: fmt.Sprintf(`
%s

The %s subcommand downloads an app from a GitHub release, verifies it
and installs it in the local system.

After verifying the artifact, %s picks the best way to install the
app: if the release only publishes a binary for the local platform, it gets
installed into the binaries directory (--bin-dir, /usr/local/bin by default).
If the release only ships a package matching the system's package format
(rpm, deb, apk), drop installs it using the package manager.

When both a binary and a package are available, %s first checks if
the app is already installed as a package (to keep it managed by the package
manager) and otherwise asks which one to install. Use --type to force a
choice without prompting:

  drop install --type=package github.com/org/repo

Installing to system locations usually requires elevated privileges: drop
shells out to sudo, which may ask for your password.

`, DropBanner("Download, verify and install apps from GitHub releases"), w2("install"), w2("drop install"), w2("drop install")),
		Use:               "install",
		Example:           fmt.Sprintf(`%s install github.com/app/repo`, appname),
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
				return fmt.Errorf("creating dropper: %w", err)
			}

			installOpts := []drop.FuncGetOption{
				drop.WithTransferTimeOut(opts.Timeout),
				drop.WithVerifyDownloads(!opts.Insecure),
				drop.WithDownloadType(opts.InstallType),
				drop.WithBinDir(opts.BinDir),
			}

			// When running interactively (and no type was forced), let the
			// user choose between a binary and a package with a prompt.
			if opts.InstallType == "" &&
				isatty.IsTerminal(os.Stdin.Fd()) && isatty.IsTerminal(os.Stdout.Fd()) {
				installOpts = append(installOpts, drop.WithArtifactSelector(huhSelector()))
			}

			// Run the installation:
			if err := dropper.Install(asset, installOpts...); err != nil {
				if errors.Is(err, drop.ErrOnlyArchives) || errors.Is(err, drop.ErrNoInstallableArtifact) {
					return fmt.Errorf("%w (try downloading with \"drop get\")", err)
				}
				return fmt.Errorf("error installing: %w", err)
			}
			return nil
		},
	}
	opts.AddFlags(attCmd)
	parentCmd.AddCommand(attCmd)
}

// huhSelector returns an artifact selector that prompts the user to choose
// between the install candidates using the arrow keys.
func huhSelector() drop.ArtifactSelector {
	return func(candidates []*drop.InstallArtifact) (*drop.InstallArtifact, error) {
		huhOpts := make([]huh.Option[*drop.InstallArtifact], 0, len(candidates))
		for _, candidate := range candidates {
			label := "Binary"
			if candidate.Kind == drop.ArtifactPackage {
				label = strings.ToUpper(candidate.PackageFormat) + " package"
			}
			huhOpts = append(huhOpts, huh.NewOption(label, candidate))
		}

		var chosen *drop.InstallArtifact
		if err := huh.NewSelect[*drop.InstallArtifact]().
			Title("How do you want to install it?").
			Options(huhOpts...).
			Value(&chosen).
			Run(); err != nil {
			return nil, fmt.Errorf("reading user selection: %w", err)
		}
		return chosen, nil
	}
}
