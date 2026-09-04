// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// flagGroupAnnotation is the flag annotation carrying the group a flag is
// rendered under in the usage text.
const flagGroupAnnotation = "drop_group"

// flagGroup is a topical bucket of flags rendered under its own heading in
// --help, after the command's own flags. Empty groups are skipped.
type flagGroup struct {
	ID    string
	Title string
}

// commandGroups holds the ordered group list of each command using grouped
// usage. Commands are built once at startup, so a package map keyed by
// command is enough.
var commandGroups = map[*cobra.Command][]flagGroup{}

// registerFlagGroups records the ordered group list for cmd and installs
// the grouped usage template on it. Flags without a group render first,
// under the regular "Flags:" heading.
func registerFlagGroups(cmd *cobra.Command, groups ...flagGroup) {
	commandGroups[cmd] = groups
	cmd.SetUsageTemplate(groupedUsageTemplate)
}

// groupFlags tags the named persistent flags of cmd with a group. Unknown
// names are skipped so a renamed flag never breaks the help.
func groupFlags(cmd *cobra.Command, group string, names ...string) {
	for _, name := range names {
		if f := cmd.PersistentFlags().Lookup(name); f != nil {
			tagFlag(f, group)
		}
	}
}

// groupFlagsByPrefix tags every persistent flag of cmd whose name starts
// with one of the prefixes. It covers flags registered by a library, such
// as the signer options, which cannot be tagged as they are added.
func groupFlagsByPrefix(cmd *cobra.Command, group string, prefixes ...string) {
	cmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		for _, p := range prefixes {
			if strings.HasPrefix(f.Name, p) {
				tagFlag(f, group)
				return
			}
		}
	})
}

func tagFlag(f *pflag.Flag, group string) {
	if f.Annotations == nil {
		f.Annotations = map[string][]string{}
	}
	f.Annotations[flagGroupAnnotation] = []string{group}
}

// groupedFlagUsages renders the local flags of cmd: the ungrouped ones
// under "Flags:" followed by each registered group under its heading. Each
// bucket is rendered by pflag so the columns match cobra's own output.
func groupedFlagUsages(cmd *cobra.Command) string {
	groups := commandGroups[cmd]
	if len(groups) == 0 {
		return cmd.LocalFlags().FlagUsages()
	}

	buckets := make(map[string]*pflag.FlagSet, len(groups))
	for _, g := range groups {
		buckets[g.ID] = pflag.NewFlagSet(g.ID, pflag.ContinueOnError)
	}
	own := pflag.NewFlagSet("flags", pflag.ContinueOnError)

	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		if vals, ok := f.Annotations[flagGroupAnnotation]; ok && len(vals) > 0 {
			if fs, ok := buckets[vals[0]]; ok {
				fs.AddFlag(f)
				return
			}
		}
		own.AddFlag(f)
	})

	var sb strings.Builder
	emit := func(title string, fs *pflag.FlagSet) {
		if !fs.HasFlags() {
			return
		}
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(title)
		sb.WriteString("\n")
		sb.WriteString(fs.FlagUsages())
	}
	emit("Flags:", own)
	for _, g := range groups {
		emit(g.Title, buckets[g.ID])
	}
	return strings.TrimRight(sb.String(), "\n")
}

func init() {
	cobra.AddTemplateFunc("groupedFlagUsages", groupedFlagUsages)
}

// groupedUsageTemplate is cobra's default usage template with the "Flags:"
// block replaced by the grouped rendering. Everything else (usage line,
// aliases, examples, subcommands, global flags, help topics) is unchanged.
const groupedUsageTemplate = `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available Commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Additional Commands:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

{{groupedFlagUsages . | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command]{{if .HasAvailableInheritedFlags}} [--help]{{end}}" for more information about a command.{{end}}
`
