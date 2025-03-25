// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package drop

const (
	EventObjectAsset        = "asset"
	EventObjectPolicy       = "policy"
	EventObjectVerification = "verification"

	EventVerbDone    = "done"
	EventVerbGet     = "get"
	EventVerbRunning = "running"
	EventVerbSaved   = "saved"
	EventVerbSkipped = "skipped"
)

type Event struct {
	Object string
	Verb   string
	Data   map[string]string
}

func (e *Event) GetDataField(field string) string {
	if _, ok := e.Data[field]; ok {
		return e.Data[field]
	}
	return ""
}

// ProgressListener is an object that reacts to the events from the
// downloader.
type ProgressListener interface {
	HandleEvent(event *Event)
}

// NoopListener is a listener tht just swallows events without doing anything
type NoopListener struct{}

func (*NoopListener) HandleEvent(event *Event) {
}
