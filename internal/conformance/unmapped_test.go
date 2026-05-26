// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

package conformance_test

import (
	"bytes"
	"encoding/json"
	"testing"

	ocsf "github.com/hstern/go-ocsf"
	"github.com/hstern/go-ocsf/events/iam"
)

// TestUnmapped_PassthroughBytes is the OCSF-25 conformance
// gate. OCSF defines `unmapped` as the wire-format
// extension carrier: a top-level field every event class
// inherits from base_event for arbitrary publisher-specific
// data the schema doesn't claim to model. The library's
// design pins unmapped's Go type to json.RawMessage so the
// contents round-trip byte-stably; this test verifies that
// promise end-to-end against a typed event class.
//
// The property: an event with an `unmapped` block containing
// nested objects, arrays, mixed-type values, and unusual key
// names round-trips through Parse → Marshal with the
// unmapped sub-document semantically identical to the
// input. "Semantically" because the surrounding event is
// Go-struct-marshaled (so key order shifts to alphabetical)
// but the unmapped subdocument is a json.RawMessage and
// its internal byte layout MUST survive.
func TestUnmapped_PassthroughBytes(t *testing.T) {
	cases := []struct {
		name         string
		unmappedJSON string
	}{
		{
			name:         "flat key/value",
			unmappedJSON: `{"vendor_field":"value","other":42}`,
		},
		{
			name:         "nested object",
			unmappedJSON: `{"deep":{"k1":"v1","k2":{"k3":"v3"}}}`,
		},
		{
			name:         "array values",
			unmappedJSON: `{"tags":["alpha","beta","gamma"],"counts":[1,2,3]}`,
		},
		{
			name:         "mixed types and bools and nulls",
			unmappedJSON: `{"flag":true,"absent":null,"score":3.14,"label":"x"}`,
		},
		{
			name:         "keys with unusual characters",
			unmappedJSON: `{"vendor.namespace.field":"v","x-custom":"c"}`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Build an Authentication event with required
			// fields populated so Validate would pass (we
			// don't actually call Validate here — the test
			// is about the unmapped passthrough — but the
			// event still needs to round-trip cleanly).
			input := authWithUnmapped([]byte(c.unmappedJSON))

			// Parse via the registry so the dispatch path
			// also gets exercised.
			ev, err := ocsf.Parse(input)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			a, ok := ev.(*iam.Authentication)
			if !ok {
				t.Fatalf("Parse returned %T, want *iam.Authentication", ev)
			}

			// The unmapped field on the struct holds the raw
			// bytes. They must equal the input bytes
			// byte-for-byte — the test of byte-stability on
			// the open-extension carrier.
			if !bytes.Equal(a.Unmapped, []byte(c.unmappedJSON)) {
				t.Errorf("unmapped bytes drifted:\n  in:  %s\n  out: %s", c.unmappedJSON, a.Unmapped)
			}

			// Marshal the full event; the unmapped subdocument
			// must survive intact inside the marshaled output.
			out, err := json.Marshal(a)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if !bytes.Contains(out, []byte(c.unmappedJSON)) {
				t.Errorf("marshal output missing unmapped subdocument verbatim:\n  unmapped expected: %s\n  marshaled: %s", c.unmappedJSON, out)
			}
		})
	}
}

// TestUnmapped_AbsentIsOmitted asserts the omitempty
// discipline: an event with no unmapped field doesn't emit
// an unmapped key on the wire. Otherwise every marshal
// output would carry an empty `"unmapped":null` which would
// fail byte-stability against fixtures that omit the field.
func TestUnmapped_AbsentIsOmitted(t *testing.T) {
	// authWithUnmapped builds the JSON; pass nil to skip the
	// field entirely.
	input := authWithUnmapped(nil)

	ev, err := ocsf.Parse(input)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if bytes.Contains(out, []byte(`"unmapped"`)) {
		t.Errorf("marshal output contains an unmapped key when input had none:\n  in:  %s\n  out: %s", input, out)
	}
}

// authWithUnmapped builds an Authentication event JSON with
// the required fields populated (so Parse + the surrounding
// validate path stay clean) and an optional unmapped block
// inserted verbatim. nil unmapped skips the field.
func authWithUnmapped(unmapped []byte) []byte {
	body := `{"category_uid":3,"class_uid":3002,"activity_id":1,"severity_id":1,"metadata":{"product":{"name":"test"},"version":"1.3.0"},"time":1618524549901,"type_uid":300201,"user":{"name":"alice"},"service":{"name":"ldap"},"cloud":{"provider":"aws"},"osint":[{}]`
	if unmapped != nil {
		body += `,"unmapped":` + string(unmapped)
	}
	body += `}`
	return []byte(body)
}
