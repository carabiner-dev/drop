// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/carabiner-dev/drop/pkg/drop"
	"github.com/carabiner-dev/drop/pkg/inventory"
)

func testRecords() []*inventory.Record {
	const binary, pkg = "unpack", "tool"
	when := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	return []*inventory.Record{
		{Host: "github.com", Org: "carabiner-dev", Repo: binary, Name: binary, Version: "v0.2.0", Kind: "binary", BinPath: "/usr/local/bin/" + binary, Verified: true, UpdatedAt: when},
		{Host: "git.example.com", Org: "acme", Repo: pkg, Name: pkg, Version: "v1.0.0", Kind: "package", PackageFormat: "rpm", UpdatedAt: when},
	}
}

// renderListing renders a table as drop would when piped and returns its
// lines, checking the shape every listing shares.
func renderListing(t *testing.T, s string, titles ...string) []string {
	t.Helper()
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("listing too short:\n%s", s)
	}
	for i, ln := range lines {
		if !strings.HasPrefix(ln, "  ") {
			t.Errorf("line %d is not indented: %q", i, ln)
		}
	}
	for _, title := range titles {
		if !strings.Contains(lines[0], title) {
			t.Errorf("header %q lacks %q", lines[0], title)
		}
	}
	if !strings.Contains(lines[1], "─") {
		t.Errorf("line under the header is not a rule: %q", lines[1])
	}
	return lines
}

func TestInstalledTable(t *testing.T) {
	out := installedTable(testRecords(), false).String()
	lines := renderListing(t, out, "NAME", "VERSION", "KIND", "LOCATION", "VERIFIED", "UPDATED", "REPOSITORY")
	for _, want := range []string{"unpack", "/usr/local/bin/unpack", "✔", "carabiner-dev/unpack", "rpm package", "✘", "git.example.com/acme/tool"} {
		if !strings.Contains(out, want) {
			t.Errorf("listing lacks %q:\n%s", want, out)
		}
	}
	if len(lines) != 4 {
		t.Errorf("want header, rule and 2 rows, got %d lines:\n%s", len(lines), out)
	}
}

func TestUpdatesTable(t *testing.T) {
	records := testRecords()
	updates := []*drop.UpdateStatus{
		{Record: records[0], LatestVersion: "v0.3.0"},
		{Record: records[1], LatestVersion: "v1.1.0"},
	}
	out := updatesTable(updates, false).String()
	renderListing(t, out, "NAME", "VERSION", "KIND", "REPOSITORY")
	for _, want := range []string{"v0.2.0 → v0.3.0", "binary", "carabiner-dev/unpack", "package (unverified)", "git.example.com/acme/tool"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary lacks %q:\n%s", want, out)
		}
	}
	t.Log("\n" + out)
}

func TestRemovalsTable(t *testing.T) {
	out := removalsTable(testRecords(), false).String()
	lines := renderListing(t, out, colName, colVersion, colKind, colLocation, colRepository)
	require.Len(t, lines, 4, "header, rule and one row per app")
	require.NotContains(t, out, colVerified, "the removal summary does not repeat the verification state")
}
