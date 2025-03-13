// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"errors"
	"fmt"

	"github.com/carabiner-dev/drop/pkg/github"
	"github.com/spf13/cobra"
)

type lsOptions struct {
	AppUrl string
	Long   bool
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

			asset := github.NewAssetFromURLString(opts.AppUrl)
			if asset == nil {
				return fmt.Errorf("unable to parse url: %q", opts.AppUrl)
			}
			client, err := github.New()
			if err != nil {
				return err
			}
			list, err := client.ListReleaseAsset(asset)
			if err != nil {
				return err
			}

			for _, a := range list {
				fmt.Println(a.Name)
			}
			return nil
		},
	}
	opts.AddFlags(lsCmd)
	parentCmd.AddCommand(lsCmd)
}
