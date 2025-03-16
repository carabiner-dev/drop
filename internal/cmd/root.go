// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"sigs.k8s.io/release-utils/log"
	"sigs.k8s.io/release-utils/version"
)

const appname = "drop"

var rootCmd = &cobra.Command{
	Long:              fmt.Sprintf(`%s: securely install software from GitHub`, appname),
	Short:             fmt.Sprintf("%s: securely install software from GitHub", appname),
	Use:               appname,
	SilenceUsage:      false,
	PersistentPreRunE: initLogging,
	Example: fmt.Sprintf(`
drop is a utility to install, update and download software from GitHub with
focused on security. Drop uses the AMPEL policy engine to verifu the integrity
of the binaries and packages you download and well as their supply chain. 

	%s snap --ver REPO=example spec.yaml
	`, appname),
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
