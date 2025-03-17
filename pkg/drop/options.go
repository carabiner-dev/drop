// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package drop

var defaultOptions = Options{}

type Options struct {
	PolicyRepository string
}

type FuncOption func(*Dropper) error

func WithPolicyRepository(repoURL string) FuncOption {
	return func(d *Dropper) error {
		d.Options.PolicyRepository = repoURL
		return nil
	}
}
