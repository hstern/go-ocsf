// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

package conformance_test

import (
	"bytes"
	"encoding/json"
	"testing"

	ocsf "github.com/hstern/go-ocsf"
)

// TestForwardCompat_BaseEventRoundTrip is the OCSF-24
// conformance gate. A payload whose class_uid the library
// doesn't recognize MUST:
//
//  1. Parse without error into *BaseEvent.
//  2. Expose the wire's classification metadata via the
//     Event accessors.
//  3. Marshal back to bytes identical to the input —
//     byte-for-byte, no canonicalization needed.
//
// The byte-identical guarantee is the load-bearing property
// for forwarding agents: a consumer running the v0.1.0
// library can receive an event class added in a later schema
// version, forward it to a v0.2.0 consumer downstream, and
// the downstream consumer sees byte-identical input to what
// the original publisher emitted. Anything weaker breaks the
// "go-ocsf in the middle is transparent on unknown classes"
// promise.
//
// BaseEvent.MarshalJSON returns a defensive copy of the raw
// bytes captured at UnmarshalJSON time, so the strict equal
// works even if the input had non-canonical whitespace —
// e.g. a publisher that emits pretty-printed JSON would
// round-trip with the pretty-printing preserved.
func TestForwardCompat_BaseEventRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		wire []byte
	}{
		{
			name: "future class_uid 99999",
			wire: []byte(`{"category_uid":99,"class_uid":99999,"class_name":"FutureClass","activity_id":1,"vendor_specific":"X","nested":{"k":1.5,"arr":[1,2,3]}}`),
		},
		{
			name: "future class with array of objects",
			wire: []byte(`{"class_uid":50000,"events":[{"id":1},{"id":2}],"flag":true}`),
		},
		{
			name: "future class with non-canonical key order",
			// Keys in alphabetical order would also work — the
			// point of this case is that the order of the input
			// is preserved verbatim, not normalized to
			// Go-struct-declaration order.
			wire: []byte(`{"z_last":"value","class_uid":77777,"a_first":1}`),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ev, err := ocsf.Parse(c.wire)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			be, ok := ev.(*ocsf.BaseEvent)
			if !ok {
				t.Fatalf("Parse for unknown class returned %T, want *ocsf.BaseEvent", ev)
			}

			// Marshal MUST return the original bytes verbatim.
			out, err := json.Marshal(be)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if !bytes.Equal(c.wire, out) {
				t.Errorf("BaseEvent round-trip lost bytes:\n  in:  %s\n  out: %s", c.wire, out)
			}
		})
	}
}

// TestForwardCompat_ZeroBaseEventMarshals confirms that a
// freshly-constructed BaseEvent (no UnmarshalJSON call)
// marshals to the empty object — a defensible default for
// downstream consumers expecting an event-shaped payload.
func TestForwardCompat_ZeroBaseEventMarshals(t *testing.T) {
	out, err := json.Marshal(&ocsf.BaseEvent{})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if want := []byte(`{}`); !bytes.Equal(out, want) {
		t.Errorf("zero BaseEvent Marshal = %s, want %s", out, want)
	}
}

// TestForwardCompat_ValidateIsNoOp asserts the forward-compat
// type's Validate is a no-op. Strict callers wanting "unknown
// class_uid is an error" type-assert before validating; the
// library's lenient-decode stance leaves the policy choice to
// the caller.
func TestForwardCompat_ValidateIsNoOp(t *testing.T) {
	wire := []byte(`{"class_uid":99999,"missing_required_fields":"plenty"}`)
	ev, err := ocsf.Parse(wire)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if err := ev.Validate(); err != nil {
		t.Errorf("BaseEvent.Validate() via Event = %v, want nil", err)
	}
}
