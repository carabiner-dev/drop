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
	AppUrl string
	Long   bool
	All    bool
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
}

func addLs(parentCmd *cobra.Command) {
	opts := &lsOptions{}
	lsCmd := &cobra.Command{
		Short:             "ls list release assets",
		Use:               "ls",
		Example:           fmt.Sprintf(`%s ls github.com/app/repo`, appname),
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

			var out io.Writer
			out = os.Stdout

			// If the URL has a version, then we list a release
			if asset.GetVersion() != "" || asset.GetVersion() == "latest" {
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
			} else {
				releases, err := client.ListReleases(asset)
				if err != nil {
					return err
				}

				return eng.RenderRepoReleases(out, asset, releases)
			}
		},
	}
	opts.AddFlags(lsCmd)
	parentCmd.AddCommand(lsCmd)
}
