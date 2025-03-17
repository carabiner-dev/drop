// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"errors"
	"fmt"

	"github.com/carabiner-dev/drop/pkg/drop"
	"github.com/carabiner-dev/drop/pkg/github"
	"github.com/carabiner-dev/drop/pkg/system"
	"github.com/spf13/cobra"
)

type getOptions struct {
	AppUrl     string
	Platform   string
	PolicyRepo string
}

// Validates the options in context with arguments
func (io *getOptions) Validate() error {
	errs := []error{}
	if io.AppUrl == "" {
		errs = append(errs, errors.New("app url not set"))
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
}

func addGet(parentCmd *cobra.Command) {
	opts := &getOptions{}
	attCmd := &cobra.Command{
		Short:             "downloads a installer or other asset and verifies it",
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

			dropper, err := drop.New(
				drop.WithPolicyRepository(opts.PolicyRepo),
			)
			if err != nil {
				return fmt.Errorf("cerating dropper: %w", err)
			}

			if err := dropper.Get(asset, "."); err != nil {
				return fmt.Errorf("error downloading: %w", err)
			}
			return nil
		},
	}
	opts.AddFlags(attCmd)
	parentCmd.AddCommand(attCmd)
}
