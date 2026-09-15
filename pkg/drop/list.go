// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package drop

import (
	"fmt"
	"maps"
	"slices"

	"github.com/carabiner-dev/drop/pkg/inventory"
)

// ListInstalled returns the apps installed with drop as recorded in the
// inventory, sorted by repository and name. It reads no network data.
func (dropper *Dropper) ListInstalled() ([]*inventory.Record, error) {
	inv, err := inventory.Open()
	if err != nil {
		return nil, fmt.Errorf("opening install inventory: %w", err)
	}
	return sortedRecords(inv), nil
}

// sortedRecords returns the inventory records in a stable order: by their
// key, which is the repository followed by the app name.
func sortedRecords(inv *inventory.Inventory) []*inventory.Record {
	records := make([]*inventory.Record, 0, len(inv.Installs))
	for _, key := range slices.Sorted(maps.Keys(inv.Installs)) {
		records = append(records, inv.Installs[key])
	}
	return records
}
