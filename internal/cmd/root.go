// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"sigs.k8s.io/release-utils/log"
	"sigs.k8s.io/release-utils/version"

	"github.com/carabiner-dev/drop/pkg/drop"
	"github.com/carabiner-dev/drop/pkg/github"
)

const (
	appname = "drop"
	arr     = `↘`

	// flagPolicyRepo and flagInsecure name the flags mentioned in messages
	flagPolicyRepo = "--policy-repo"
	flagInsecure   = "--insecure"
)

// noPolicyAppliesMessage explains that the publisher's policies exist but
// none applies to the release being installed (their conditions skipped
// them all), so nothing was verified, and lists the ways to proceed.
func noPolicyAppliesMessage(subcommand string, asset *github.Asset) string {
	slug := asset.Org + "/" + asset.Repo
	return fmt.Sprintf(`
  ❌ %s

  %s found policies for this project, but every one of them declared it
  does not apply to release %s, so nothing was verified. The publisher
  may not cover this version yet. You have two options:

    1. Use policies from another repository or a local checkout:
         %s %s %s=<repo> %s

    2. Skip verification (not recommended):
         %s %s %s %s
`,
		w(fmt.Sprintf("No verification policy applies to %s %s", slug, asset.Version)), appname, asset.Version,
		appname, subcommand, flagPolicyRepo, slug,
		appname, subcommand, flagInsecure, slug,
	)
}

// noPolicyMessage explains that an artifact cannot be verified because its
// publisher has no policies, and lists the ways to proceed: requesting
// community policies, using another policy source or skipping verification.
// The subcommand name is used to build the example commands.
func noPolicyMessage(subcommand string, asset *github.Asset, policyRepo string) string {
	slug := asset.Org + "/" + asset.Repo
	community := ""
	if policyRepo == "" {
		policyRepo = drop.DefaultPolicyRepository(asset.Host, asset.Org)
		community = fmt.Sprintf("\n    %s (%s/)", drop.CommunityPolicyRepositoryURL, drop.CommunityPolicyPath(asset.Org, asset.Repo))
	}
	return fmt.Sprintf(`
  ❌ %s

  %s verifies every artifact against its publisher's policies before
  installing it, and this project has none yet. Looked in:
    %s (%s)%s
  You have three options:

    1. Ask for community policies to be written for it:
         %s request %s

    2. Use policies from another repository or a local checkout:
         %s %s %s=<repo> %s

    3. Skip verification (not recommended):
         %s %s %s %s
`,
		w(fmt.Sprintf("No verification policies found for %s", slug)), appname, policyRepo, drop.PolicyPathsLabel(asset.Repo), community,
		appname, slug,
		appname, subcommand, flagPolicyRepo, slug,
		appname, subcommand, flagInsecure, slug,
	)
}

var (
	w  = color.New(color.FgHiWhite, color.BgBlack).SprintFunc()
	w2 = color.New(color.Faint, color.FgWhite, color.BgBlack).SprintFunc()
)

func AmpelBanner(legend string) string {
	r := color.New(color.FgRed, color.BgBlack).SprintFunc()
	y := color.New(color.FgYellow, color.BgBlack).SprintFunc()
	g := color.New(color.FgGreen, color.BgBlack).SprintFunc()
	w := color.New(color.FgHiWhite, color.BgBlack).SprintFunc()
	w2 := color.New(color.Faint, color.FgWhite, color.BgBlack).SprintFunc()
	if legend != "" {
		legend = w2(": " + legend)
	}
	return fmt.Sprintf("%s%s%s%s%s", r("⬤"), y("⬤"), g("⬤"), w("AMPEL"), legend)
}

func DropBanner(legend string) string {
	w2 := color.New(color.Faint, color.FgWhite, color.BgBlack).SprintFunc()
	if legend != "" {
		legend = w2(": " + legend)
	}
	return fmt.Sprintf("↘️ %s%s", w(appname), legend)
}

var rootCmd = &cobra.Command{
	Long: fmt.Sprintf(`
%s

%s is a utility to install, update and download software from GitHub with
focused on security. Drop uses the %s policy engine to verify the
integrity of the binaries and packages you download, as well as their supply
chain metadata. 

`, DropBanner("securely install software from GitHub"), appname, AmpelBanner("")),
	Short:             fmt.Sprintf("%s: securely install software from GitHub", appname),
	Use:               appname,
	SilenceUsage:      false,
	PersistentPreRunE: initLogging,
	Example: fmt.Sprintf(`
drop is a utility to install, update and download software from GitHub with
focused on security. Drop uses the AMPEL policy engine to verify the integrity
of the binaries and packages you download and well as their supply chain. 

%s

List assets in a release:

  %s ls -l github.com/org/repo

List assets withour grouping into "installables":

  %s ls -l --all github.com/org/repo

%s

Download and verify artifacts from a GitHub release:

  %s get github.com/org/repo@latest

Download and verify a specific file from a release:

  %s get github.com/org/repo#checksums.txt

%s

Install a binary from a release:

  %s install github.com/org/repo

Install the same, but using a system package:

  %s install --package github.com/org/repo

	`, w2("LISTING ASSETS"), appname, appname, w2("DOWNLOAD"), appname, appname, w2("INSTALLING"), appname, appname),
}

type commandLineOptions struct {
	logLevel string
}

var commandLineOpts = commandLineOptions{}

func initLogging(*cobra.Command, []string) error {
	return log.SetupGlobalLogger(commandLineOpts.logLevel)
}

// Execute builds the command
func Execute() {
	rootCmd.PersistentFlags().StringVar(
		&commandLineOpts.logLevel,
		"log-level", "info", fmt.Sprintf("the logging verbosity, either %s", log.LevelNames()),
	)
	addInstall(rootCmd)
	addLs(rootCmd)
	addGet(rootCmd)
	addCheckUpdate(rootCmd)
	addUpdate(rootCmd)
	addRequest(rootCmd)
	rootCmd.AddCommand(version.WithFont("doom"))

	if err := rootCmd.Execute(); err != nil {
		logrus.Fatal(err)
	}
}
