// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"errors"
	"fmt"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"sigs.k8s.io/release-utils/version"

	"github.com/carabiner-dev/drop/pkg/drop"
	"github.com/carabiner-dev/drop/pkg/github"
)

type requestOptions struct {
	AppUrl     string
	PolicyRepo string
	Print      bool
}

// Validate checks the options
func (ro *requestOptions) Validate() error {
	if ro.AppUrl == "" {
		return errors.New("no repository specified")
	}
	return nil
}

// AddFlags adds the subcommand flags
func (ro *requestOptions) AddFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVar(
		&ro.PolicyRepo, "policy-repo", "", "policy source that was checked, when not the default",
	)
	cmd.PersistentFlags().BoolVarP(
		&ro.Print, "print", "p", false, "print the issue link instead of opening it in the browser",
	)
}

func addRequest(parentCmd *cobra.Command) {
	opts := &requestOptions{}
	requestCmd := &cobra.Command{
		Short: "Request community policies for a repository",
		Long: fmt.Sprintf(`
%s

%s verifies artifacts against the policies their publisher defines. When
a project has none, the %s subcommand asks the drop project to write
community policies for it by opening a prefilled issue in the drop repository:

  %s

Before filing, %s checks that the repository exists and records
its latest release in the issue. The issue is filed from your browser, where
you are signed in to GitHub, so no token is needed. When not running on a
terminal (or with --print) the link is printed instead, ready to paste into
a browser.

`, DropBanner("Request community policies for a repository"), appname, w2("request"), drop.DropRepositoryURL+"/issues", appname),
		Use:               "request",
		Example:           fmt.Sprintf(`%s request github.com/org/repo`, appname),
		SilenceUsage:      false,
		SilenceErrors:     true,
		PersistentPreRunE: initLogging,
		PreRunE: func(_ *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.AppUrl = args[0]
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := opts.Validate(); err != nil {
				return err
			}
			cmd.SilenceUsage = true

			asset := github.NewAssetFromURLString(opts.AppUrl)
			if asset == nil || asset.Org == "" || asset.Repo == "" {
				return fmt.Errorf("unable to parse repository: %q", opts.AppUrl)
			}
			if asset.Host == "" {
				asset.Host = github.DefaultHost
			}

			// Make sure the repository exists and find the release drop
			// would check for policies, so the request points at it.
			client, err := github.New()
			if err != nil {
				return fmt.Errorf("creating GitHub client: %w", err)
			}
			repo, err := client.GetRepository(asset)
			if err != nil {
				if errors.Is(err, github.ErrRepositoryNotFound) {
					return fmt.Errorf("%w (check the spelling: https://%s/%s/%s)", err, asset.Host, asset.Org, asset.Repo)
				}
				return err
			}
			release, err := client.LatestRelease(repo)
			if err != nil {
				return fmt.Errorf("%w (policies verify releases, so the repository needs at least one)", err)
			}

			info := version.GetVersionInfo()
			request := &drop.PolicyRequest{
				Host:             repo.Host,
				Org:              repo.Org,
				Repo:             repo.Repo,
				PolicyRepository: opts.PolicyRepo,
				Release:          release.GetVersion(),
				Version:          info.GitVersion,
				Platform:         info.Platform,
			}
			link := request.URL()

			if opts.Print || !interactive() {
				fmt.Println(link)
				return nil
			}

			fmt.Printf("  ✨ %s\n", w(fmt.Sprintf("Requesting community policies for %s (release %s)", request.Slug(), request.Release)))
			fmt.Printf("  🌐 %s\n", w("Opening the issue in your browser..."))
			if err := browser.OpenURL(link); err != nil {
				fmt.Printf("      ⚠️  could not open a browser (%v), file the request here:\n\n  %s\n\n", err, link)
				return nil
			}
			fmt.Printf("      ✔️  done. If nothing opened, file the request here:\n\n  %s\n\n", link)
			return nil
		},
	}
	opts.AddFlags(requestCmd)
	parentCmd.AddCommand(requestCmd)
}
