// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

// Package inventory keeps track of the artifacts drop installs in the local
// system. The data lives in a single versioned JSON document in the user's
// configuration directory and records, for every installed app, what was
// installed and enough metadata to later verify, update or uninstall it.
package inventory

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Version is the current schema version of the inventory file.
const Version = 1

const dirName = "drop"

// FileName is the name of the inventory database file.
const FileName = "installed.json"

// Inventory is the database of artifacts installed by drop.
type Inventory struct {
	Version  int                `json:"version"`
	Installs map[string]*Record `json:"installs"`

	path string
}

// Record stores the data of one installed app.
type Record struct {
	// Repository coordinates and installable name identifying the app.
	Host string `json:"host"`
	Org  string `json:"org"`
	Repo string `json:"repo"`
	Name string `json:"name"`

	// Version is the release tag the installed artifact came from.
	Version string `json:"version"`

	// Kind is the artifact type that was installed (binary or package).
	Kind string `json:"kind"`

	// Asset is the exact release asset that was downloaded.
	Asset string `json:"asset"`

	// Digest captures the hashes of the verified artifact, keyed by
	// algorithm. For binaries it can be checked against the installed
	// file, for packages integrity checks are delegated to the package
	// manager and the digest ties the record to the verified file.
	Digest map[string]string `json:"digest,omitempty"`

	// BinPath is the path where the binary was installed (binaries only).
	BinPath string `json:"binPath,omitempty"`

	// PackageFormat is the package type handed to the package manager
	// (packages only).
	PackageFormat string `json:"packageFormat,omitempty"`

	// Verified records if the artifact passed policy verification or was
	// installed with verification disabled.
	Verified bool `json:"verified"`

	InstalledAt time.Time `json:"installedAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Key returns the string keying the record in the inventory.
func (r *Record) Key() string {
	return fmt.Sprintf("%s/%s/%s#%s", r.Host, r.Org, r.Repo, r.Name)
}

// DefaultPath returns the location of the inventory database in the user's
// configuration directory.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolving user configuration directory: %w", err)
	}
	return filepath.Join(dir, dirName, FileName), nil
}

// Open loads the inventory from its default location, returning an empty
// inventory if no database exists yet.
func Open() (*Inventory, error) {
	path, err := DefaultPath()
	if err != nil {
		return nil, err
	}
	return OpenFile(path)
}

// OpenFile loads the inventory from a file, returning an empty inventory
// bound to the path if the file does not exist yet.
func OpenFile(path string) (*Inventory, error) {
	inv := &Inventory{
		Version:  Version,
		Installs: map[string]*Record{},
		path:     path,
	}

	data, err := os.ReadFile(path) //nolint:gosec // reading the inventory is the point
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return inv, nil
		}
		return nil, fmt.Errorf("reading inventory: %w", err)
	}

	if err := json.Unmarshal(data, inv); err != nil {
		return nil, fmt.Errorf("parsing inventory: %w", err)
	}

	if inv.Version > Version {
		return nil, fmt.Errorf("inventory version %d is newer than the supported version %d", inv.Version, Version)
	}

	if inv.Installs == nil {
		inv.Installs = map[string]*Record{}
	}
	return inv, nil
}

// Save atomically writes the inventory back to the file it was loaded from.
func (inv *Inventory) Save() error {
	if inv.path == "" {
		return errors.New("inventory is not bound to a file")
	}

	dir := filepath.Dir(inv.path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("creating inventory directory: %w", err)
	}

	data, err := json.MarshalIndent(inv, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling inventory: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".installed-*.json")
	if err != nil {
		return fmt.Errorf("creating temporary file: %w", err)
	}

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()           //nolint:errcheck
		_ = os.Remove(tmp.Name()) //nolint:errcheck
		return fmt.Errorf("writing inventory: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name()) //nolint:errcheck
		return fmt.Errorf("closing inventory file: %w", err)
	}

	if err := os.Rename(tmp.Name(), inv.path); err != nil {
		_ = os.Remove(tmp.Name()) //nolint:errcheck
		return fmt.Errorf("replacing inventory: %w", err)
	}
	return nil
}

// Add upserts a record into the inventory. When the app is already recorded,
// the original installation timestamp is preserved.
func (inv *Inventory) Add(record *Record) {
	now := time.Now().UTC()
	key := record.Key()
	if existing, ok := inv.Installs[key]; ok && !existing.InstalledAt.IsZero() {
		record.InstalledAt = existing.InstalledAt
	} else if record.InstalledAt.IsZero() {
		record.InstalledAt = now
	}
	record.UpdatedAt = now
	inv.Installs[key] = record
}

// Get returns the record stored under a key or nil if there is none.
func (inv *Inventory) Get(key string) *Record {
	return inv.Installs[key]
}

// Remove deletes the record stored under a key, returning true if it existed.
func (inv *Inventory) Remove(key string) bool {
	_, ok := inv.Installs[key]
	delete(inv.Installs, key)
	return ok
}
