// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/carabiner-dev/termtable"
	"github.com/spf13/cobra"

	"github.com/carabiner-dev/drop/internal/notifier"
	"github.com/carabiner-dev/drop/pkg/drop"
	"github.com/carabiner-dev/drop/pkg/inventory"
)

type rmOptions struct {
	Yes   bool
	Quiet bool
}

// AddFlags adds the subcommands flags
func (ro *rmOptions) AddFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().BoolVarP(
		&ro.Yes, "yes", "y", false, "never prompt: remove without confirmation",
	)

	cmd.PersistentFlags().BoolVarP(
		&ro.Quiet, "quiet", "q", false, "less verbose output (for scripts, etc)",
	)
}

func addRm(parentCmd *cobra.Command) {
	opts := &rmOptions{}
	rmCmd := &cobra.Command{
		Short: "removes apps installed with drop",
		Long: fmt.Sprintf(`
%s

The %s subcommand uninstalls apps installed with drop: it deletes the
binary (or removes the package through the system's package manager, with
sudo when needed) and drops the app from drop's inventory.

Apps are named as %s shows them. A repository (org/repo) names every
app installed from it:

  drop rm cosign
  drop rm sigstore/cosign

Before removing anything, %s lists what will be removed and asks for
confirmation. Use --yes/-y to skip the prompt.

`, DropBanner("Remove apps installed with drop"), w2("rm"), w2("drop list"), w2("drop rm")),
		Use:               "rm app [app...]",
		Aliases:           []string{"remove", "uninstall"},
		Example:           fmt.Sprintf("%s rm cosign", appname),
		Args:              cobra.MinimumNArgs(1),
		SilenceUsage:      false,
		SilenceErrors:     true,
		PersistentPreRunE: initLogging,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			var lstnr drop.ProgressListener = &notifier.Listener{}
			if opts.Quiet {
				lstnr = &drop.NoopListener{}
			}

			dropper, err := drop.New(drop.WithListener(lstnr))
			if err != nil {
				return fmt.Errorf("creating dropper: %w", err)
			}

			records, err := resolveRemovals(dropper, args)
			if err != nil {
				return err
			}

			// Show what goes away before touching anything
			fmt.Println("\nRemoving:")
			if _, err := removalsTable(records, interactive()).WriteTo(os.Stdout); err != nil {
				return fmt.Errorf("rendering removal summary: %w", err)
			}
			fmt.Printf("\n%d app(s) will be removed.\n", len(records))

			if !opts.Yes && !confirm() {
				fmt.Println("Operation aborted.")
				return nil
			}

			errs := []error{}
			for _, record := range records {
				fmt.Printf("\n🗑️  Removing %s %s:\n", w(record.Name), record.Version)
				if err := dropper.Remove(record); err != nil {
					fmt.Printf("  ❌ removing %s failed: %v\n", record.Name, err)
					errs = append(errs, fmt.Errorf("removing %s: %w", record.Name, err))
				}
			}

			if len(errs) > 0 {
				return fmt.Errorf("%d of %d removals failed: %w", len(errs), len(records), errors.Join(errs...))
			}
			fmt.Printf("\n  ✨ %d app(s) removed!\n", len(records))
			return nil
		},
	}
	opts.AddFlags(rmCmd)
	parentCmd.AddCommand(rmCmd)
}

// resolveRemovals turns the app references given on the command line into
// inventory records, each once. A name shared by apps from several
// repositories must be qualified with its repository.
func resolveRemovals(dropper *drop.Dropper, refs []string) ([]*inventory.Record, error) {
	records := []*inventory.Record{}
	seen := map[string]bool{}
	for _, ref := range refs {
		found, err := dropper.FindInstalled(ref)
		if err != nil {
			return nil, err
		}
		if !strings.Contains(ref, "/") && len(found) > 1 {
			repos := make([]string, 0, len(found))
			for _, r := range found {
				repos = append(repos, recordRepository(r))
			}
			return nil, fmt.Errorf(
				"%s is installed from several repositories (%s), name the repository to remove from",
				ref, strings.Join(repos, ", "),
			)
		}
		for _, r := range found {
			if seen[r.Key()] {
				continue
			}
			seen[r.Key()] = true
			records = append(records, r)
		}
	}
	return records, nil
}

// removalsTable renders the apps about to be removed as a listing.
func removalsTable(records []*inventory.Record, toTerminal bool) *termtable.Table {
	titles := []string{colName, colVersion, colKind, colLocation, colRepository}
	rows := make([][]string, 0, len(records))
	for _, r := range records {
		rows = append(rows, []string{r.Name, r.Version, r.Kind, recordLocation(r), recordRepository(r)})
	}
	t := listingTable(titles, rows, toTerminal)
	t.Column(1).SetAlign(termtable.AlignRight)
	return t
}
