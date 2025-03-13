// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

type installOptions struct {
	AppUrl string
}

// Validates the options in context with arguments
func (io *installOptions) Validate() error {
	errs := []error{}
	if io.AppUrl == "" {
		errs = append(errs, errors.New("app url not set"))
	}

	return errors.Join(errs...)
}

// AddFlags adds the subcommands flags
func (io *installOptions) AddFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(
		&io.AppUrl, "app", "a", "", "app to install",
	)
}

func addInstall(parentCmd *cobra.Command) {
	opts := &installOptions{}
	attCmd := &cobra.Command{
		Short:             "installs a binary or package after verifying it",
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
			return nil
		},
	}
	opts.AddFlags(attCmd)
	parentCmd.AddCommand(attCmd)
}
