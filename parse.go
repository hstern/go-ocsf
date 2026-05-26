// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

package ocsf

import (
	"encoding/json"
	"fmt"
	"sync"
)

// ClassFactory returns a freshly-zeroed [Event] of a specific
// OCSF class. The registry calls it when [Parse] dispatches an
// incoming payload to the matching class.
//
// Factories MUST return a pointer-to-struct (so json.Unmarshal
// can populate the fields). Codegen-emitted factories always
// satisfy this; consumers writing their own factories for
// custom event types should mirror the pattern.
type ClassFactory func() Event

var (
	classRegistryMu sync.RWMutex
	classRegistry   = map[int]ClassFactory{} //nolint:gochecknoglobals // process-wide dispatch table
)

// RegisterClass registers factory under the given OCSF
// class_uid. The library's generated event classes call this
// from package init() so that [Parse] can dispatch incoming
// payloads to the right type without the consumer importing
// every events/<category>/ subpackage explicitly — importing
// any one of them, or a wildcard via the helper docs in the
// README, side-effects the registration through init().
//
// Re-registering a class_uid that's already in the registry
// panics. Codegen never emits the same class_uid twice, and a
// duplicate from outside the library is a programming error
// worth surfacing loudly — a silent overwrite would let a
// stray "test" registration shadow the real type and produce
// quiet decoding bugs in production.
//
// RegisterClass is safe for concurrent use, but the natural
// call site is package init() where no concurrency exists yet.
func RegisterClass(classUID int, factory ClassFactory) {
	if factory == nil {
		panic(fmt.Sprintf("ocsf: RegisterClass(%d): factory must not be nil", classUID))
	}
	classRegistryMu.Lock()
	defer classRegistryMu.Unlock()
	if _, exists := classRegistry[classUID]; exists {
		panic(fmt.Sprintf("ocsf: RegisterClass: duplicate registration for class_uid %d", classUID))
	}
	classRegistry[classUID] = factory
}

// LookupClass returns the registered factory for classUID and
// reports whether one exists. Exposed for diagnostics and for
// consumers building their own dispatch logic; the canonical
// entry point is [Parse].
func LookupClass(classUID int) (ClassFactory, bool) {
	classRegistryMu.RLock()
	defer classRegistryMu.RUnlock()
	f, ok := classRegistry[classUID]
	return f, ok
}

// Parse decodes a JSON-encoded OCSF event into the right
// typed [Event].
//
// The dispatch reads class_uid from data, looks up the
// matching factory via the registry, and unmarshals the
// payload into the factory's freshly-zeroed struct. When the
// class_uid is missing or unknown to the registry, Parse falls
// back to [BaseEvent] — preserving the raw bytes for
// byte-stable round-trip and the four classification accessors
// for inspection.
//
// The forward-compat fallback is intentional and not an error:
// a publisher running a newer schema than this library can
// still hand OCSF events to a Go consumer, and the consumer
// chooses what to do with the BaseEvent (forward as-is,
// re-parse with a newer library, log + drop). Callers that
// want strict mode — "an unknown class_uid is an error" —
// type-assert the return:
//
//	e, err := ocsf.Parse(data)
//	if err != nil { ... }
//	if _, ok := e.(*ocsf.BaseEvent); ok {
//	    return fmt.Errorf("unknown class_uid %d", e.OCSFClassUID())
//	}
//
// Parse does NOT validate the decoded event against the OCSF
// schema; that's Validate's job (lands in Phase 4). The
// lenient-unmarshal / strict-marshal stance from the design
// doc applies: whatever the wire delivered, we decode it.
func Parse(data []byte) (Event, error) {
	classUID, hadField, err := peekClassUID(data)
	if err != nil {
		return nil, fmt.Errorf("ocsf: Parse: peek class_uid: %w", err)
	}
	if hadField {
		if factory, ok := LookupClass(classUID); ok {
			e := factory()
			if err := json.Unmarshal(data, e); err != nil {
				return nil, fmt.Errorf("ocsf: Parse: class_uid=%d: %w", classUID, err)
			}
			return e, nil
		}
	}
	// Missing class_uid or unknown to the registry —
	// forward-compat fallback.
	var b BaseEvent
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("ocsf: Parse: BaseEvent fallback: %w", err)
	}
	return &b, nil
}

// peekClassUID does the cheap "what's the class_uid" pass over
// the input without decoding the rest of the payload. Returns
// the value plus a hadField boolean so callers can distinguish
// "missing" (route to BaseEvent) from "present and zero"
// (still route to BaseEvent at v1.3.0 — class_uid=0 is the
// base_event marker, and BaseEvent is the right Go-side
// representation).
func peekClassUID(data []byte) (classUID int, hadField bool, err error) {
	var probe struct {
		ClassUID *int `json:"class_uid"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return 0, false, err
	}
	if probe.ClassUID == nil {
		return 0, false, nil
	}
	return *probe.ClassUID, true, nil
}
