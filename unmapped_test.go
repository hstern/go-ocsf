// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

package ocsf

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestUnmapped_RoundTripsBytesVerbatim(t *testing.T) {
	// The whole point of Unmapped is byte-stable round-trip of
	// opaque JSON: encode whatever the wire had, decode it back
	// out the same way. Key order, whitespace, numeric precision
	// — all preserved.
	in := []byte(`{"vendor_field":"value","nested":{"k":1.5,"arr":[1,2,3]}}`)
	var u Unmapped = in
	out, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("json.Marshal(Unmapped): %v", err)
	}
	if !bytes.Equal(in, out) {
		t.Errorf("Unmapped round-trip lost bytes:\n  in:  %s\n  out: %s", in, out)
	}
}

func TestUnmapped_UnmarshalIntoSurroundingStruct(t *testing.T) {
	type evt struct {
		Class    string   `json:"class"`
		Unmapped Unmapped `json:"unmapped,omitempty"`
	}
	wire := []byte(`{"class":"Authentication","unmapped":{"src_specific":42}}`)
	var got evt
	if err := json.Unmarshal(wire, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Class != "Authentication" {
		t.Errorf("Class = %q, want Authentication", got.Class)
	}
	if want := []byte(`{"src_specific":42}`); !bytes.Equal(got.Unmapped, want) {
		t.Errorf("Unmapped = %s, want %s", got.Unmapped, want)
	}
}

func TestJSONNull(t *testing.T) {
	if string(JSONNull) != "null" {
		t.Errorf("JSONNull = %q, want %q", JSONNull, "null")
	}
	type evt struct {
		Field Unmapped `json:"field"` // no omitempty
	}
	out, err := json.Marshal(evt{Field: JSONNull})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if want := `{"field":null}`; string(out) != want {
		t.Errorf("Marshal evt{Field:JSONNull} = %s, want %s", out, want)
	}
}
