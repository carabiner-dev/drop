// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/carabiner-dev/drop/pkg/drop"
)

func addCheckUpdate(parentCmd *cobra.Command) {
	attCmd := &cobra.Command{
		Short: "checks if the apps installed with drop have new releases",
		Long: fmt.Sprintf(`
%s

The %s subcommand goes through the apps installed with drop and
checks their GitHub repositories to see if any of them has published a newer
release.

The data of the installed apps is read from drop's inventory, only apps
installed through %s are checked.

`, DropBanner("Check installed apps for new releases"), w2("check-update"), w2("drop install")),
		Use:               "check-update",
		Aliases:           []string{"check-updates"},
		Example:           fmt.Sprintf("%s check-update", appname),
		SilenceUsage:      false,
		SilenceErrors:     true,
		PersistentPreRunE: initLogging,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SilenceUsage = true

			dropper, err := drop.New()
			if err != nil {
				return fmt.Errorf("creating dropper: %w", err)
			}

			statuses, err := dropper.CheckUpdates()
			if err != nil {
				return err
			}

			if len(statuses) == 0 {
				fmt.Println("  📭 No apps installed with drop yet.")
				return nil
			}

			updates := 0
			failed := 0
			for _, status := range statuses {
				repo := fmt.Sprintf("%s/%s/%s", status.Record.Host, status.Record.Org, status.Record.Repo)
				switch {
				case status.Error != nil:
					failed++
					fmt.Printf("  ⚠️  %s (%s): check failed: %v\n", w(status.Record.Name), repo, status.Error)
				case status.UpdateAvailable:
					updates++
					fmt.Printf("  ⬆️  %s %s → %s (%s)\n", w(status.Record.Name), status.Record.Version, status.LatestVersion, repo)
				default:
					fmt.Printf("  ✔️  %s %s is up to date\n", w(status.Record.Name), status.Record.Version)
				}
			}

			fmt.Println()
			switch {
			case updates > 0:
				fmt.Printf("  %d of %d apps can be updated\n", updates, len(statuses))
			case failed == 0:
				fmt.Println("  ✨ Everything is up to date!")
			}
			if failed > 0 {
				return fmt.Errorf("could not check %d of %d apps", failed, len(statuses))
			}
			return nil
		},
	}
	parentCmd.AddCommand(attCmd)
}
