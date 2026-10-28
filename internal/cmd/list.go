// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"os"

	"github.com/carabiner-dev/termtable"
	"github.com/spf13/cobra"

	"github.com/carabiner-dev/drop/pkg/drop"
	"github.com/carabiner-dev/drop/pkg/github"
	"github.com/carabiner-dev/drop/pkg/inventory"
)

func addList(parentCmd *cobra.Command) {
	listCmd := &cobra.Command{
		Short: "lists the apps installed with drop",
		Long: fmt.Sprintf(`
%s

The %s subcommand prints the apps installed with drop as recorded in
its inventory: the version installed, how it was installed (a bare binary, a
binary extracted from an archive, or a system package), where it went, whether
the artifact passed policy verification (✔) or was installed with --insecure
(✘), and when it was installed or last updated. Nothing is fetched from
GitHub; see %s for that.

`, DropBanner("List the apps installed with drop"), w2("list"), w2("drop check-update")),
		Use:               "list",
		Aliases:           []string{"installed"},
		Example:           fmt.Sprintf("%s list", appname),
		SilenceUsage:      false,
		SilenceErrors:     true,
		PersistentPreRunE: initLogging,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SilenceUsage = true

			dropper, err := drop.New()
			if err != nil {
				return fmt.Errorf("creating dropper: %w", err)
			}
			records, err := dropper.ListInstalled()
			if err != nil {
				return err
			}
			if len(records) == 0 {
				fmt.Println("  📭 No apps installed with drop yet.")
				return nil
			}

			if _, err := installedTable(records, interactive()).WriteTo(os.Stdout); err != nil {
				return fmt.Errorf("rendering installed apps: %w", err)
			}
			fmt.Printf("\n  %d app(s) installed with drop\n", len(records))
			return nil
		},
	}
	parentCmd.AddCommand(listCmd)
}

// installedTable renders the inventory records as a listing with the
// version column aligned to the right and the verification mark centered.
func installedTable(records []*inventory.Record, toTerminal bool) *termtable.Table {
	titles := []string{colName, colVersion, colKind, colLocation, colVerified, colUpdated, colRepository}
	rows := make([][]string, 0, len(records))
	for _, r := range records {
		rows = append(rows, []string{
			r.Name, r.Version, r.Kind, recordLocation(r), recordVerified(r),
			r.UpdatedAt.Local().Format("2006-01-02"), recordRepository(r),
		})
	}
	t := listingTable(titles, rows, toTerminal)
	t.Column(1).SetAlign(termtable.AlignRight)
	t.Column(4).SetAlign(termtable.AlignCenter)
	return t
}

// recordRepository renders the repository an app came from in short form,
// leaving the host out when it is github.com.
func recordRepository(r *inventory.Record) string {
	slug := r.Org + "/" + r.Repo
	if r.Host == "" || r.Host == github.DefaultHost {
		return slug
	}
	return r.Host + "/" + slug
}

// recordLocation describes where an installed app lives: the binary path,
// or the package manager that owns it.
func recordLocation(r *inventory.Record) string {
	if r.BinPath != "" {
		return r.BinPath
	}
	if r.PackageFormat != "" {
		return r.PackageFormat + " package"
	}
	return "-"
}

// recordVerified renders whether the artifact passed policy verification
// when it was installed.
func recordVerified(r *inventory.Record) string {
	if r.Verified {
		return "✔"
	}
	return "✘"
}
