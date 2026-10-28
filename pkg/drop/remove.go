// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package drop

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/carabiner-dev/drop/pkg/inventory"
)

// ErrNotInstalled is returned when an app reference names nothing in the
// inventory of apps installed with drop.
var ErrNotInstalled = errors.New("app not installed with drop")

// FindInstalled returns the inventory records an app reference names, in
// inventory order. A reference is the name of an installed app, or the
// repository it came from as org/repo or host/org/repo (optionally with a
// scheme), which names every app installed from it.
func (dropper *Dropper) FindInstalled(ref string) ([]*inventory.Record, error) {
	inv, err := inventory.Open()
	if err != nil {
		return nil, fmt.Errorf("opening install inventory: %w", err)
	}
	records := matchInstalled(inv, ref)
	if len(records) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrNotInstalled, ref)
	}
	return records, nil
}

// matchInstalled returns the records in the inventory that ref names, by
// app name or by repository.
func matchInstalled(inv *inventory.Inventory, ref string) []*inventory.Record {
	ref = strings.TrimSpace(ref)
	if i := strings.Index(ref, "://"); i >= 0 {
		ref = ref[i+3:]
	}
	ref = strings.Trim(ref, "/")
	if ref == "" {
		return nil
	}

	records := []*inventory.Record{}
	for _, key := range slices.Sorted(maps.Keys(inv.Installs)) {
		r := inv.Installs[key]
		switch ref {
		case r.Name, r.Org + "/" + r.Repo, r.Host + "/" + r.Org + "/" + r.Repo:
			records = append(records, r)
		}
	}
	return records
}

// Remove uninstalls an app installed with drop, deleting its binary or
// removing its package through the package manager, and drops it from the
// inventory. The inventory is left untouched when the removal fails.
func (dropper *Dropper) Remove(record *inventory.Record, funcs ...FuncGetOption) error {
	opts := defaultGetOptions
	opts.Options = dropper.Options

	for _, fn := range funcs {
		if err := fn(&opts); err != nil {
			return err
		}
	}
	if opts.Listener == nil {
		opts.Listener = &NoopListener{}
	}

	if err := dropper.impl.RemoveInstalled(&opts, record); err != nil {
		return err
	}
	if err := dropper.impl.ForgetInstall(&opts, record); err != nil {
		return fmt.Errorf("%s removed, but updating the inventory failed: %w", record.Name, err)
	}
	return nil
}

// ForgetInstall deletes an app's record from the inventory. A record that
// is already gone is not an error.
func (di *defaultImplementation) ForgetInstall(_ *GetOptions, record *inventory.Record) error {
	inv, err := di.openInventory()
	if err != nil {
		return err
	}
	if !inv.Remove(record.Key()) {
		return nil
	}
	return inv.Save()
}
