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

// TestRoundTrip_NoFieldLost is the v0.1.x-shipping form of
// the byte-stable round-trip gate. For every vendored
// fixture, Parse → Marshal MUST preserve every field present
// on the wire: same key path, same value, no loss. The
// codegen output may add zero-valued keys on the marshal
// side — required-field schema markers translate to
// `json:"name"` (no omitempty) on the Go struct, and a
// fixture that doesn't carry every nested-required field
// produces additional empty/zero entries on the marshal
// side.
//
// The "round-trip is byte-identical" form of this gate is
// deferred to OCSF-29 (1.8.0 + submodule pivot), which
// audits fixture completeness against the upstream schema
// and decides whether the codegen should soften
// required-field emission to omitempty universally. Both
// directions are reasonable and the trade-off interacts
// with what 1.8.0 changes; bundling the decision with the
// schema bump avoids throwaway work on 1.3.0 fixtures.
//
// Validate runs alongside as a smoke test — every fixture
// must Validate clean. The forward-compat fixture under
// future/ parses into BaseEvent (Validate returns nil by
// design); the strict-bytes form of forward-compat
// round-trip lives in forward_compat_test.go (OCSF-24) and
// IS byte-identical because BaseEvent preserves the raw
// bytes verbatim.
func TestRoundTrip_NoFieldLost(t *testing.T) {
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
			var inMap any
			if decErr := decodeForCompare(in, &inMap); decErr != nil {
				t.Fatalf("decode input: %v", decErr)
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
			var outMap any
			if decErr := decodeForCompare(out, &outMap); decErr != nil {
				t.Fatalf("decode output: %v", decErr)
			}

			if missing := subsetMissing("", inMap, outMap); missing != "" {
				t.Errorf("round-trip dropped input field(s):\n  first missing path: %s\n  in:  %s\n  out: %s", missing, in, out)
			}
		})
		return nil
	})
	if walkErr != nil {
		t.Fatalf("WalkDir: %v", walkErr)
	}
}

// decodeForCompare decodes JSON into a generic any tree
// using json.Number for numerics so integer precision
// (class_uid, timestamp) doesn't drift through float64.
func decodeForCompare(b []byte, out *any) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	return dec.Decode(out)
}

// subsetMissing checks that every leaf path in `in` exists
// with the same value in `out`. Returns "" when in ⊆ out;
// returns a dotted path identifying the first missing or
// mismatched leaf otherwise. `out` may have additional keys
// — those are tolerated (codegen emits required-but-empty
// fields where the wire omits them; the OCSF-29 cutover
// will decide whether to soften that).
func subsetMissing(prefix string, in, out any) string {
	switch tin := in.(type) {
	case map[string]any:
		tout, ok := out.(map[string]any)
		if !ok {
			return prefix
		}
		keys := make([]string, 0, len(tin))
		for k := range tin {
			keys = append(keys, k)
		}
		sortStrings(keys)
		for _, k := range keys {
			p := k
			if prefix != "" {
				p = prefix + "." + k
			}
			if _, present := tout[k]; !present {
				return p
			}
			if r := subsetMissing(p, tin[k], tout[k]); r != "" {
				return r
			}
		}
		return ""
	case []any:
		tout, ok := out.([]any)
		if !ok {
			return prefix
		}
		if len(tin) != len(tout) {
			return prefix + "[]"
		}
		for i := range tin {
			if r := subsetMissing(prefix+"[]", tin[i], tout[i]); r != "" {
				return r
			}
		}
		return ""
	default:
		ainBytes, _ := json.Marshal(in)
		aoutBytes, _ := json.Marshal(out)
		if !bytes.Equal(ainBytes, aoutBytes) {
			return prefix
		}
		return ""
	}
}

// sortStrings is a tiny insertion sort to avoid importing
// the sort package for this one site. Key counts at any
// object level are small.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
