// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/carabiner-dev/drop/pkg/github"
	"github.com/carabiner-dev/drop/pkg/render"
	"github.com/carabiner-dev/drop/pkg/render/drivers"
	"github.com/spf13/cobra"
)

type lsOptions struct {
	AppUrl       string
	Long         bool
	All          bool
	ListReleases bool
}

// Validates the options in context with arguments
func (lo *lsOptions) Validate() error {
	errs := []error{}
	if lo.AppUrl == "" {
		errs = append(errs, errors.New("github url not set"))
	}

	return errors.Join(errs...)
}

// AddFlags adds the subcommands flags
func (lo *lsOptions) AddFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().BoolVarP(
		&lo.Long, "long", "l", false, "list in long format",
	)

	cmd.PersistentFlags().BoolVarP(
		&lo.All, "all", "a", false, "don't consolidate assets into installables",
	)

	cmd.PersistentFlags().BoolVarP(
		&lo.ListReleases, "releases", "r", false, "list releases in the repo instead of artifacts",
	)
}

func addLs(parentCmd *cobra.Command) {
	opts := &lsOptions{}
	lsCmd := &cobra.Command{
		Short: "list release assets",
		Long: fmt.Sprintf(`
%s

%s

The ls subcommand works like its unix counterpart. It lets you list artifacts
published as assets in a github release. To list artifacts, just pass it a 
repository reference:

  %s ls org/repo

If you want to see extra details about the assets, pass it the -l|--long switch:

  %s ls -l org/repo

The output will then include columns of data about each of the artifacts, example:

%s ls -l sigstore/cosign

total 5
📄➖➖➖➖➖➖➖  sigstore-bot  sigstore  3906      Feb    19   13:56  cosign_checksums.txt              
📄➖➖➖➖➖➖➖  sigstore-bot  sigstore  1424      Feb    19   13:56  cosign_checksums.txt-keyless.pem  
📄➖➖➖➖➖➖➖  sigstore-bot  sigstore  96        Feb    19   13:56  cosign_checksums.txt-keyless.sig  
📄➖➖➖➖➖➖➖  sigstore-bot  sigstore  178       Feb    19   13:56  release-cosign.pub                
💾🐧🍏🪟📦➖➖➖  sigstore-bot  sigstore  48504720  Feb    19   13:55  cosign    

%s

The first column of the ls -l output contains data about the artifacts encoded
in emoji indicators:

📄/💾 Indicates if the asset is just a file or an "installable", a collection
of artifacts for different os / architectures. The installable entry may be
grouping togethe other files related to the asset, such as SBOMs, keys, or other 
security metadata.

🐧🍏🪟 These are the platform indicators. They show that a release has published
artifacts for Linux / MacOS / Windows.

📦 Means that the installable has system packages available (rpm, deb, dmsg, msi, etc)

🎁 This means that the release has archives published (zip, tar, bz2, etc)

`, DropBanner("List software releases and published assets"), w("LISTING ARTIFACTS"), appname, appname, appname, w("EMOJI INDICATORS")),
		Use: "ls [flags] github.com/org/reposository",
		Example: fmt.Sprintf(`List all assets in the latest release:
  %s ls github.com/app/repo
 
Using -a|--all lists all assets without grouping them into an "installable":

  %s ls -la github.com/app/repo

You can list the assets in a specific release:

  %s ls -la github.com/app/repo@v1.2.0

You can use @latest (or simply leave it blank) to list the latest release. To
see a list of the available releases, use the -r|--releases switch:

  %s ls -lr github.com/app/repo

  `, appname, appname, appname, appname),
		SilenceUsage:      false,
		SilenceErrors:     true,
		PersistentPreRunE: initLogging,
		PreRunE: func(_ *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.AppUrl = args[0]
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
				return fmt.Errorf("unable to parse url: %q", opts.AppUrl)
			}

			// Creat the GitHub client
			client, err := github.New()
			if err != nil {
				return err
			}

			// Init the rendering engine
			drv := drivers.NewLsTTY()
			drv.Options.Long = opts.Long

			eng, err := render.New(
				render.WithDriver(drv),
			)
			if err != nil {
				return err
			}

			var out io.Writer = os.Stdout

			// If the URL has a version, then we list a release
			if opts.ListReleases {
				releases, err := client.ListReleases(asset)
				if err != nil {
					return err
				}

				return eng.RenderRepoReleases(out, asset, releases)
			} else {
				if opts.All {
					list, err := client.ListReleaseAssets(asset)
					if err != nil {
						return err
					}
					return eng.RenderReleaseAssets(out, asset, list)
				} else {
					list, err := client.ListReleaseInstallables(asset)
					if err != nil {
						return err
					}
					return eng.RenderReleaseInstallables(out, asset, list)
				}
			}
		},
	}
	opts.AddFlags(lsCmd)
	parentCmd.AddCommand(lsCmd)
}
