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
)

// TestRoundTrip_ByteStable is the primary correctness gate
// for the codegen output. For every vendored fixture, the
// Parse → Marshal cycle MUST produce JSON semantically
// identical to the input. "Semantic" here means the
// canonicalized form — keys sorted, whitespace stripped — and
// is compared with bytes.Equal after canonicalization.
//
// Why canonicalize rather than raw bytes.Equal? The fixtures
// are hand-curated JSON; their key order doesn't match the
// codegen-emitted struct declaration order (which is
// alphabetical), so a raw byte compare would fail on a key
// that's perfectly preserved. Canonicalization decodes each
// side to a map[string]any and re-marshals; encoding/json
// sorts map keys, so equivalent JSON canonicalizes
// identically.
//
// The property being tested IS that no fields are lost,
// fields are present with the correct values, and no spurious
// fields are added. Key order on the wire is not (and
// shouldn't be) part of the OCSF contract; the codegen's
// alphabetical-by-field-name emission is a stable convenience,
// not a wire requirement.
//
// Validate runs alongside as a smoke test — every fixture
// must Validate clean too. The forward-compat fixture under
// future/ parses into BaseEvent (Validate returns nil by
// design) and its bytes round-trip verbatim via
// BaseEvent.MarshalJSON, so the same property holds.
func TestRoundTrip_ByteStable(t *testing.T) {
	fixtures := specfixtures.V130()
	walkErr := fs.WalkDir(fixtures, "v1.3.0", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".json") {
			return err
		}
		t.Run(path, func(t *testing.T) {
			in, err := fs.ReadFile(fixtures, path)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			canonIn, err := canonicalize(in)
			if err != nil {
				t.Fatalf("canonicalize input: %v", err)
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

// canonicalize decodes b as generic JSON and re-marshals it;
// the resulting bytes have keys in alphabetical order and no
// insignificant whitespace, which is the semantic-equality
// form the byte-stable test compares against.
//
// Goes through map[string]any so that nested objects also
// sort recursively. Numeric precision is preserved on
// integers but not necessarily on floats — encoding/json's
// default decoder uses float64 for all numbers, and the
// fixtures intentionally use integer wire values where the
// schema's type is integer (avoids round-trip float drift on
// the OCSF time field).
func canonicalize(b []byte) ([]byte, error) {
	var v any
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber() // preserve integer precision on large class_uids and timestamps
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return json.Marshal(v)
}
