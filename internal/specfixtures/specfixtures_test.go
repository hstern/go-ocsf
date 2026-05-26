// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

package specfixtures_test

import (
	"io/fs"
	"strings"
	"testing"

	ocsf "github.com/hstern/go-ocsf"
	"github.com/hstern/go-ocsf/internal/specfixtures"
)

// TestFixturesParseAndValidate walks the embedded v1.8.0 fixture
// tree and confirms every JSON file Parses into the expected
// shape and Validates without error. The forward-compat
// `future/` subdirectory parses into BaseEvent (whose Validate
// is a no-op) — that asymmetry is documented in the README.
//
// This is a smoke test that exercises Parse + Validate
// end-to-end against vendored payloads; the dedicated
// byte-stable round-trip property test lands in OCSF-23 with
// the full conformance scaffold.
func TestFixturesParseAndValidate(t *testing.T) {
	fixtures := specfixtures.V180()
	var fixtureCount int
	err := fs.WalkDir(fixtures, "v1.8.0", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".json") {
			return nil
		}
		t.Run(path, func(t *testing.T) {
			data, err := fs.ReadFile(fixtures, path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			e, err := ocsf.Parse(data)
			if err != nil {
				t.Fatalf("Parse %s: %v", path, err)
			}
			if e == nil {
				t.Fatalf("Parse %s: nil event with nil error", path)
			}
			// Validate must pass: minimal fixtures carry every
			// required field, full fixtures carry siblings and
			// constraint members. The forward-compat fixture
			// under future/ lands as a BaseEvent whose Validate
			// returns nil by design.
			if err := e.Validate(); err != nil {
				t.Errorf("Validate %s: %v", path, err)
			}
		})
		fixtureCount++
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir: %v", err)
	}
	// Sanity: ensure the walk found the curated set rather
	// than silently passing over an empty tree (e.g. an embed
	// regression).
	if fixtureCount < 5 {
		t.Errorf("found %d fixtures, want >= 5 (one per curated shape)", fixtureCount)
	}
}

// TestFixturesIncludeForwardCompat asserts the future/
// fixture lands as a BaseEvent — the forward-compat fallback
// is the property OCSF-24 will test more thoroughly.
func TestFixturesIncludeForwardCompat(t *testing.T) {
	data, err := fs.ReadFile(specfixtures.V180(), "v1.8.0/future/unknown_class-9999.json")
	if err != nil {
		t.Fatalf("read forward-compat fixture: %v", err)
	}
	e, err := ocsf.Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	be, ok := e.(*ocsf.BaseEvent)
	if !ok {
		t.Fatalf("forward-compat fixture dispatched to %T, want *ocsf.BaseEvent", e)
	}
	if be.OCSFClassUID() != 99999 {
		t.Errorf("class_uid = %d, want 99999", be.OCSFClassUID())
	}
}
