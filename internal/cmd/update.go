// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/carabiner-dev/drop/internal/notifier"
	"github.com/carabiner-dev/drop/pkg/drop"
)

type updateOptions struct {
	Yes   bool
	Quiet bool
}

// AddFlags adds the subcommands flags
func (uo *updateOptions) AddFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().BoolVarP(
		&uo.Yes, "yes", "y", false, "update without asking for confirmation",
	)

	cmd.PersistentFlags().BoolVarP(
		&uo.Quiet, "quiet", "q", false, "less verbose output (for scripts, etc)",
	)
}

func addUpdate(parentCmd *cobra.Command) {
	opts := &updateOptions{}
	attCmd := &cobra.Command{
		Short: "updates the apps installed with drop to their latest releases",
		Long: fmt.Sprintf(`
%s

The %s subcommand checks the apps installed with drop for newer
releases and reinstalls those with updates available, reusing the choices
made when each app was installed: the artifact kind (binary or package),
the binaries directory and the verification settings.

Before updating, %s prints a summary of the pending updates and
asks for confirmation. Use --yes/-y to skip the confirmation prompt:

  drop update -y

`, DropBanner("Update the apps installed with drop"), w2("update"), w2("drop update")),
		Use:               "update",
		Example:           fmt.Sprintf("%s update", appname),
		SilenceUsage:      false,
		SilenceErrors:     true,
		PersistentPreRunE: initLogging,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SilenceUsage = true

			var lstnr drop.ProgressListener = &notifier.Listener{}
			if opts.Quiet {
				lstnr = &drop.NoopListener{}
			}

			dropper, err := drop.New(drop.WithListener(lstnr))
			if err != nil {
				return fmt.Errorf("creating dropper: %w", err)
			}

			statuses, err := dropper.CheckUpdates()
			if err != nil {
				return err
			}

			updates := []*drop.UpdateStatus{}
			for _, status := range statuses {
				if status.Error != nil {
					fmt.Printf("  ⚠️  %s: update check failed: %v\n", w(status.Record.Name), status.Error)
					continue
				}
				if status.UpdateAvailable {
					updates = append(updates, status)
				}
			}

			if len(updates) == 0 {
				fmt.Println("  ✨ Everything is up to date!")
				return nil
			}

			// Print the update summary before touching anything
			fmt.Println("\nUpdating:")
			tw := tabwriter.NewWriter(os.Stdout, 2, 2, 2, ' ', 0)
			for _, status := range updates {
				kind := status.Record.Kind
				if !status.Record.Verified {
					kind += " (unverified)"
				}
				_, _ = fmt.Fprintf( //nolint:errcheck
					tw, "  %s\t%s → %s\t%s\t%s/%s/%s\n",
					status.Record.Name, status.Record.Version, status.LatestVersion,
					kind, status.Record.Host, status.Record.Org, status.Record.Repo,
				)
			}
			if err := tw.Flush(); err != nil {
				return fmt.Errorf("rendering update summary: %w", err)
			}
			fmt.Printf("\n%d app(s) will be updated.\n", len(updates))

			if !opts.Yes && !confirm() {
				fmt.Println("Operation aborted.")
				return nil
			}

			errs := []error{}
			for _, status := range updates {
				fmt.Printf("\n⬆️  Updating %s to %s:\n", w(status.Record.Name), status.LatestVersion)
				if err := dropper.Update(status); err != nil {
					fmt.Printf("  ❌ updating %s failed: %v\n", status.Record.Name, err)
					errs = append(errs, fmt.Errorf("updating %s: %w", status.Record.Name, err))
				}
			}

			if len(errs) > 0 {
				return fmt.Errorf("%d of %d updates failed: %w", len(errs), len(updates), errors.Join(errs...))
			}
			fmt.Printf("\n  ✨ %d app(s) updated!\n", len(updates))
			return nil
		},
	}
	opts.AddFlags(attCmd)
	parentCmd.AddCommand(attCmd)
}

// confirm asks the user to approve the pending updates, defaulting to no.
func confirm() bool {
	fmt.Print("Is this ok [y/N]: ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}
