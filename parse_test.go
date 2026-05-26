// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

package ocsf_test

import (
	"bytes"
	"encoding/json"
	"testing"

	ocsf "github.com/hstern/go-ocsf"
	// Importing events/iam side-effects 7 registrations through
	// the package's generated init() functions — enough to
	// exercise Parse for a concrete class without dragging in
	// every generated event package.
	"github.com/hstern/go-ocsf/events/iam"
)

func TestParse_KnownClassUIDDispatchesToTypedEvent(t *testing.T) {
	// Authentication: class_uid 3002. The wire payload below
	// includes the minimum classification fields plus a User
	// reference (the required field on Authentication).
	wire := []byte(`{"class_uid":3002,"class_name":"Authentication","category_uid":3,"category_name":"iam","user":{"name":"alice"}}`)
	e, err := ocsf.Parse(wire)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	a, ok := e.(*iam.Authentication)
	if !ok {
		t.Fatalf("Parse dispatched to %T, want *iam.Authentication", e)
	}
	if a.OCSFClassUID() != 3002 {
		t.Errorf("OCSFClassUID() = %d, want 3002", a.OCSFClassUID())
	}
	if a.User == nil || a.User.Name != "alice" {
		t.Errorf("User.Name = %v, want \"alice\"", a.User)
	}
}

func TestParse_UnknownClassUIDFallsBackToBaseEvent(t *testing.T) {
	wire := []byte(`{"class_uid":99999,"class_name":"FutureClass","custom":"data"}`)
	e, err := ocsf.Parse(wire)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	b, ok := e.(*ocsf.BaseEvent)
	if !ok {
		t.Fatalf("Parse for unknown class_uid returned %T, want *ocsf.BaseEvent", e)
	}
	if b.OCSFClassUID() != 99999 {
		t.Errorf("BaseEvent.OCSFClassUID() = %d, want 99999", b.OCSFClassUID())
	}
}

func TestParse_MissingClassUIDFallsBackToBaseEvent(t *testing.T) {
	wire := []byte(`{"some_field":"value"}`)
	e, err := ocsf.Parse(wire)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, ok := e.(*ocsf.BaseEvent); !ok {
		t.Fatalf("Parse for missing class_uid returned %T, want *ocsf.BaseEvent", e)
	}
}

func TestParse_UnknownClassPreservesBytes(t *testing.T) {
	// The forward-compat guarantee: a BaseEvent obtained via
	// Parse round-trips byte-stably through MarshalJSON.
	wire := []byte(`{"class_uid":99999,"class_name":"FutureClass","vendor_field":"X","nested":{"k":1.5}}`)
	e, err := ocsf.Parse(wire)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !bytes.Equal(wire, out) {
		t.Errorf("Parse->Marshal lost bytes:\n  in:  %s\n  out: %s", wire, out)
	}
}

func TestParse_InvalidJSONReturnsError(t *testing.T) {
	if _, err := ocsf.Parse([]byte(`{"class_uid": not json}`)); err == nil {
		t.Fatal("Parse(invalid JSON) returned nil error")
	}
}

func TestRegisterClass_DuplicatePanics(t *testing.T) {
	// Use a class_uid the codegen-emitted init()s have NOT
	// claimed. 1 isn't a valid class_uid (Authentication is the
	// minimum at 1001), so this should be safe today; the test
	// itself only proves duplicate detection, not that 1 is
	// permanently free.
	uid := 1
	ocsf.RegisterClass(uid, func() ocsf.Event { return &ocsf.BaseEvent{} })

	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("RegisterClass(%d) duplicate did not panic", uid)
		}
	}()
	ocsf.RegisterClass(uid, func() ocsf.Event { return &ocsf.BaseEvent{} })
}

func TestLookupClass(t *testing.T) {
	// Authentication's class_uid is 3002.
	if _, ok := ocsf.LookupClass(3002); !ok {
		t.Errorf("LookupClass(3002) not found — events/iam init() never ran?")
	}
	if _, ok := ocsf.LookupClass(99998); ok {
		t.Errorf("LookupClass(99998) found a factory, want absent")
	}
}
