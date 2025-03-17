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
)

const (
	appname = "drop"
	arr     = `↘`
)

var w = color.New(color.FgHiWhite, color.BgBlack).SprintFunc()
var w2 = color.New(color.Faint, color.FgWhite, color.BgBlack).SprintFunc()

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

`, DropBanner("securely install software from GitHub)"), appname, AmpelBanner("")),
	Short:             fmt.Sprintf("%s: securely install software from GitHub", appname),
	Use:               appname,
	SilenceUsage:      false,
	PersistentPreRunE: initLogging,
	Example: fmt.Sprintf(`
drop is a utility to install, update and download software from GitHub with
focused on security. Drop uses the AMPEL policy engine to verify the integrity
of the binaries and packages you download and well as their supply chain. 

List assets in a release:

	%s ls -l github.com/org/repo

Download and verify artifacts from a GitHub release:

	%s ls get github.com/org/repo@latest

Download and verify a specific file from a release:

	%s ls get github.com/org/repo#checksums.txt

Install a binary from a release:

	%s ls install github.com/org/repo

Install the same, but using a system package:

	%s ls install --package github.com/org/repo

	`, appname, appname, appname, appname, appname),
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
	rootCmd.AddCommand(version.WithFont("doom"))

	if err := rootCmd.Execute(); err != nil {
		logrus.Fatal(err)
	}
}
