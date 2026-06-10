// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package notifier

import (
	"fmt"
	"strconv"

	"github.com/fatih/color"

	"github.com/carabiner-dev/drop/pkg/drop"
)

var w = color.New(color.FgHiWhite, color.BgBlack).SprintFunc()

// w2 = color.New(color.Faint, color.FgWhite, color.BgBlack).SprintFunc()

type Listener struct{}

func (l *Listener) HandleEvent(event *drop.Event) {
	switch event.Object {
	case drop.EventObjectPolicy:
		switch event.Verb {
		case drop.EventVerbGet:
			repo := ""
			if s := event.GetDataField("repo"); s != "" {
				repo = fmt.Sprintf(" (source: %s)", s)
			}
			fmt.Printf("  💫 %s%s\n", w("Looking for policies"), repo)
		case drop.EventVerbDone:
			sets := "0"
			if s := event.GetDataField("count"); s != "" {
				sets = s
			}
			fmt.Printf("      ✔️  %s policy sets found\n", sets)
		}
	case drop.EventObjectAsset:
		switch event.Verb {
		case drop.EventVerbGet:
			f := "asset"
			if s := event.GetDataField("filename"); s != "" {
				f = s
			}

			size := ""
			if s := event.GetDataField("size"); s != "" {
				i, err := strconv.Atoi(s)
				if err == nil {
					size = fmt.Sprintf(" (%.2f MB)", float64(i)/1024/1024)
				}
			}
			fmt.Printf("  ⏬ %s%s\n", w(fmt.Sprintf("Downloading %s", f)), size)
		case drop.EventVerbDone:
			fmt.Println("      ✔️  done")
		case drop.EventVerbSaved:
			p := ""
			if s := event.GetDataField("path"); s != "" {
				p = fmt.Sprintf(" (written to %s)", s)
			}
			fmt.Printf("  💾 %s%s\n", w("Download complete!"), p)
		}
	case drop.EventObjectInstall:
		switch event.Verb {
		case drop.EventVerbRunning:
			sudo := ""
			if event.GetDataField("sudo") == "true" {
				sudo = " with sudo (you may be asked for your password)"
			}
			if event.GetDataField("kind") == string(drop.ArtifactPackage) {
				format := event.GetDataField("format")
				fmt.Printf("  📦 %s\n", w(fmt.Sprintf("Installing %s package%s...", format, sudo)))
			} else {
				target := event.GetDataField("target")
				fmt.Printf("  🔧 %s\n", w(fmt.Sprintf("Installing binary to %s%s...", target, sudo)))
			}
		case drop.EventVerbDone:
			name := "app"
			if s := event.GetDataField("name"); s != "" {
				name = s
			}
			fmt.Printf("  🎉 %s\n", w(fmt.Sprintf("%s installed!", name)))
		case drop.EventVerbSkipped:
			if reason := event.GetDataField("reason"); reason != "" {
				fmt.Printf("      ℹ️  %s\n", reason)
			}
		}
	case drop.EventObjectVerification:
		switch event.Verb {
		case drop.EventVerbRunning:
			fmt.Printf("  🛡️  %s\n", w("Verifying artifact..."))
		case drop.EventVerbSkipped:
			fmt.Printf("  🚫  %s\n", w("Security verification skipped"))
		case drop.EventVerbDone:
			if s := event.GetDataField("passed"); s != "" {
				if s == "true" {
					fmt.Println("      ✅  PASS")
				} else {
					fmt.Println("      ❌  FAIL")
				}
			} else {
				fmt.Println("      ✔️  done")
			}
		}
	}
}
