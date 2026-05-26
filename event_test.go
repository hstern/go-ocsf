// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

package ocsf_test

import (
	"bytes"
	"encoding/json"
	"testing"

	ocsf "github.com/hstern/go-ocsf"
	"github.com/hstern/go-ocsf/events/iam"
)

// TestEventInterface_GeneratedClassesSatisfy is the
// compile-time check that the generated event classes implement
// the Event interface from the root package. If a future
// emitter refactor breaks the method set on a generated class,
// the assignment below fails to compile.
//
// Authentication is the canonical sample (events/iam — the
// well-known class_uid 3002). If the interface needs to grow,
// every generated class needs the new method; checking
// Authentication catches the regression cheaply without
// asserting against all 72 classes.
func TestEventInterface_GeneratedClassesSatisfy(t *testing.T) {
	var _ ocsf.Event = iam.Authentication{}
}

func TestBaseEvent_UnmarshalExtractsClassification(t *testing.T) {
	wire := []byte(`{"class_uid":3002,"class_name":"Authentication","category_uid":3,"category_name":"iam","metadata":{"version":"1.3.0"}}`)
	var b ocsf.BaseEvent
	if err := json.Unmarshal(wire, &b); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got := b.OCSFClassUID(); got != 3002 {
		t.Errorf("OCSFClassUID() = %d, want 3002", got)
	}
	if got := b.OCSFClassName(); got != "Authentication" {
		t.Errorf("OCSFClassName() = %q, want Authentication", got)
	}
	if got := b.OCSFCategoryUID(); got != 3 {
		t.Errorf("OCSFCategoryUID() = %d, want 3", got)
	}
	if got := b.OCSFCategoryName(); got != "iam" {
		t.Errorf("OCSFCategoryName() = %q, want iam", got)
	}
}

func TestBaseEvent_MissingClassificationIsNotAnError(t *testing.T) {
	// A payload lacking class_uid (forward-compat with future
	// extensions, or a malformed publisher) decodes cleanly into
	// a BaseEvent whose accessors return zero values. The
	// library reports the missing data to the consumer via the
	// zero, not via an unmarshal error.
	wire := []byte(`{"some_field":"value"}`)
	var b ocsf.BaseEvent
	if err := json.Unmarshal(wire, &b); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if b.OCSFClassUID() != 0 {
		t.Errorf("OCSFClassUID() = %d, want 0 for missing field", b.OCSFClassUID())
	}
	if b.OCSFClassName() != "" {
		t.Errorf("OCSFClassName() = %q, want empty for missing field", b.OCSFClassName())
	}
}

func TestBaseEvent_RoundTripsBytesVerbatim(t *testing.T) {
	// The byte-stability guarantee: an event Parse'd into a
	// BaseEvent and Marshal'd back produces the exact same
	// bytes — key order, spacing, numeric formatting — so a
	// forwarding agent doesn't accidentally rewrite payloads
	// it doesn't understand.
	wire := []byte(`{"class_uid":9999,"class_name":"FutureClass","category_uid":99,"vendor_field":"X","nested":{"k":1.5,"arr":[1,2,3]}}`)
	var b ocsf.BaseEvent
	if err := json.Unmarshal(wire, &b); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	out, err := json.Marshal(&b)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !bytes.Equal(wire, out) {
		t.Errorf("BaseEvent round-trip lost bytes:\n  in:  %s\n  out: %s", wire, out)
	}
}

func TestBaseEvent_RawIsDefensiveCopy(t *testing.T) {
	wire := []byte(`{"class_uid":1}`)
	var b ocsf.BaseEvent
	if err := json.Unmarshal(wire, &b); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	raw := b.Raw()
	if !bytes.Equal(raw, wire) {
		t.Errorf("Raw() initial = %s, want %s", raw, wire)
	}
	// Modify the returned slice and confirm the next Raw() call
	// returns the unmodified original.
	for i := range raw {
		raw[i] = '!'
	}
	raw2 := b.Raw()
	if !bytes.Equal(raw2, wire) {
		t.Errorf("Raw() after mutation = %s, want %s (defensive copy broken)", raw2, wire)
	}
}

func TestBaseEvent_ZeroValueMarshalsToEmptyObject(t *testing.T) {
	// A BaseEvent constructed without UnmarshalJSON has no raw
	// bytes. Marshal returns the empty object rather than null
	// so it slots into a downstream stream of OCSF events
	// without producing a value the next consumer's typed
	// Unmarshal would choke on.
	var b ocsf.BaseEvent
	out, err := json.Marshal(&b)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if want := []byte(`{}`); !bytes.Equal(out, want) {
		t.Errorf("zero BaseEvent Marshal = %s, want %s", out, want)
	}
}

func TestBaseEvent_AsEventInterface(t *testing.T) {
	wire := []byte(`{"class_uid":9999,"class_name":"FutureClass","category_uid":99,"category_name":"future"}`)
	var b ocsf.BaseEvent
	if err := json.Unmarshal(wire, &b); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	// Use through the interface to confirm method dispatch works.
	var e ocsf.Event = &b
	if e.OCSFClassUID() != 9999 {
		t.Errorf("via Event: ClassUID = %d, want 9999", e.OCSFClassUID())
	}
	if e.OCSFCategoryName() != "future" {
		t.Errorf("via Event: CategoryName = %q, want future", e.OCSFCategoryName())
	}
}
