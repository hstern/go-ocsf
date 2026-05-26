// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

// Package conformance holds the Phase 5 conformance tests:
// byte-stable round-trip (OCSF-23), forward-compat / BaseEvent
// (OCSF-24), and unmapped passthrough (OCSF-25). They live
// under internal/ because they consume the embedded fixture
// tree from internal/specfixtures.
package conformance_test

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"strings"
	"testing"

	ocsf "github.com/hstern/go-ocsf"
	"github.com/hstern/go-ocsf/internal/specfixtures"

	// Blank-import every events/<category>/ subpackage so the
	// codegen-emitted init() functions register their classes
	// with the root ocsf.Parse dispatch table. Without these,
	// the registry stays empty during the conformance test and
	// every fixture routes to BaseEvent (whose Validate is a
	// no-op) — the test would then pass trivially without
	// exercising any of the codegen-emitted typed event paths.
	//
	// base_event lives at events/base/ but registers class_uid
	// 0, which Parse routes to BaseEvent regardless; importing
	// it isn't necessary for dispatch, but the blank import
	// keeps the import list complete in case future schema
	// versions land class_uid 0 as something concrete.
	_ "github.com/hstern/go-ocsf/events/application"
	_ "github.com/hstern/go-ocsf/events/base"
	_ "github.com/hstern/go-ocsf/events/discovery"
	_ "github.com/hstern/go-ocsf/events/findings"
	_ "github.com/hstern/go-ocsf/events/iam"
	_ "github.com/hstern/go-ocsf/events/network"
	_ "github.com/hstern/go-ocsf/events/remediation"
	_ "github.com/hstern/go-ocsf/events/system"
)

// TestRoundTrip_ByteStable is the strict-form OCSF-23 gate:
// for every vendored fixture, Parse → Marshal must produce
// JSON semantically identical to the input — same keys, same
// values, no fields dropped, no fields added. The comparison
// is canonical (decode-and-remarshal both sides with
// json.Number for integer precision, then bytes.Equal) so
// equivalent JSON with different key order is treated as
// equal.
//
// Why canonical rather than raw bytes.Equal? The fixtures
// are hand-curated JSON whose key order doesn't match the
// codegen-emitted struct declaration order (alphabetical).
// Canonicalization normalizes both sides; the property being
// tested is the structural / value fidelity of the codegen
// types, not byte-for-byte wire-order preservation.
//
// This is the load-bearing correctness gate for the codegen
// output. If the emitter ever drops a field, adds a spurious
// one, or mis-handles a type, this test catches it. Phase 5
// shipped the weaker subset assertion (TestRoundTrip_NoFieldLost)
// because pre-OCSF-31 non-required numerics/bools dropped
// false/0 values and pre-OCSF-32 fixtures missed required
// sub-fields. After OCSF-31 + OCSF-32 both shapes hold and
// the strict form is reachable.
//
// Validate runs alongside as a smoke test — every fixture
// must Validate clean. The forward-compat fixture under
// future/ parses into BaseEvent (Validate returns nil by
// design); its strict round-trip is asserted by the
// byte-identical comparison in forward_compat_test.go
// (OCSF-24).
func TestRoundTrip_ByteStable(t *testing.T) {
	fixtures := specfixtures.V180()
	walkErr := fs.WalkDir(fixtures, "v1.8.0", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".json") {
			return err
		}
		t.Run(path, func(t *testing.T) {
			in, err := fs.ReadFile(fixtures, path)
			if err != nil {
				t.Fatalf("read: %v", err)
			}

			ev, err := ocsf.Parse(in)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if vErr := ev.Validate(); vErr != nil {
				t.Fatalf("Validate: %v", vErr)
			}

			out, err := json.Marshal(ev)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}

			canonIn, err := canonicalize(in)
			if err != nil {
				t.Fatalf("canonicalize input: %v", err)
			}
			canonOut, err := canonicalize(out)
			if err != nil {
				t.Fatalf("canonicalize output: %v", err)
			}
			if !bytes.Equal(canonIn, canonOut) {
				t.Errorf("round-trip drift:\n  canonical in:  %s\n  canonical out: %s", canonIn, canonOut)
			}
		})
		return nil
	})
	if walkErr != nil {
		t.Fatalf("WalkDir: %v", walkErr)
	}
}

// canonicalize decodes JSON into a generic any tree (with
// json.Number so integer precision on class_uid and timestamps
// doesn't drift through float64), then re-marshals.
// encoding/json sorts map keys on marshal, so the resulting
// byte sequence is the canonical form for that JSON value:
// two semantically equivalent inputs produce identical output.
func canonicalize(b []byte) ([]byte, error) {
	var v any
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return json.Marshal(v)
}
