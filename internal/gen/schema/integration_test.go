// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

package schema_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/hstern/go-ocsf/internal/gen/schema"
)

// TestLoadUpstream is the end-to-end smoke test: read the
// repository's submodule-pinned OCSF schema and check the
// high-level counts plus a handful of spot-check invariants
// (well-known class UIDs, attribute resolution). Asserting on
// counts catches the kind of partial-load failure where a
// single directory walk misses files; asserting on specific
// classes catches misresolved extends chains.
//
// The counts below match the upstream 1.8.0 release and will
// shift with each schema-version bump (OCSF-29 onward); the
// shape of the test stays the same.
func TestLoadUpstream(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	schemaDir := filepath.Join(repoRoot, "internal", "schema", "upstream")

	s, err := schema.Load(schemaDir)
	if err != nil {
		t.Fatalf("Load(%q): %v", schemaDir, err)
	}

	if got, want := s.Version, "1.8.0"; got != want {
		t.Errorf("Version = %q, want %q", got, want)
	}
	if got, want := len(s.Categories), 8; got != want {
		t.Errorf("Categories = %d, want %d (system, findings, iam, network, discovery, application, remediation, unmanned_systems)", got, want)
	}
	if c, found := s.Categories["iam"]; !found || c.UID != 3 {
		t.Errorf("Categories[iam] = %+v, want UID 3", c)
	}
	if got := len(s.Dictionary.Attributes); got < 800 {
		// Upstream has 907 attributes at 1.8.0; allow some slack
		// for benign upstream edits but flag any dramatic loss.
		t.Errorf("Dictionary.Attributes = %d, want >= 800", got)
	}
	if got := len(s.Dictionary.Types); got != 23 {
		t.Errorf("Dictionary.Types = %d, want 23", got)
	}
	if got := len(s.Objects); got < 150 {
		t.Errorf("Objects = %d, want >= 150", got)
	}
	if got := len(s.Events); got < 70 {
		t.Errorf("Events = %d, want >= 70", got)
	}

	// Spot check: well-known class UIDs.
	auth, ok := s.Events["authentication"]
	if !ok {
		t.Fatal("Events[authentication] missing")
	}
	if got, want := s.ClassUID(auth), 3002; got != want {
		t.Errorf("ClassUID(authentication) = %d, want %d", got, want)
	}
	det, ok := s.Events["detection_finding"]
	if !ok {
		t.Fatal("Events[detection_finding] missing")
	}
	if got, want := s.ClassUID(det), 2004; got != want {
		t.Errorf("ClassUID(detection_finding) = %d, want %d", got, want)
	}

	// Spot check: extends chain. authentication extends iam, which
	// extends base_event. After resolution, authentication should
	// carry base_event's metadata attribute (came in via base_event
	// inheritance, not via authentication.json directly).
	foundMetadata := false
	for _, a := range auth.Attributes {
		if a.Name == "metadata" {
			foundMetadata = true
			if a.Requirement != "required" {
				t.Errorf("authentication.metadata.Requirement = %q, want required (from base_event)", a.Requirement)
			}
			break
		}
	}
	if !foundMetadata {
		t.Error("authentication.Attributes missing inherited metadata field")
	}

	// Spot check: dictionary type resolution. The user object's
	// `name` attribute is typed username_t locally (override of the
	// dictionary type).
	user, ok := s.Objects["user"]
	if !ok {
		t.Fatal("Objects[user] missing")
	}
	for _, a := range user.Attributes {
		if a.Name == "name" {
			if a.Type != "username_t" {
				t.Errorf("user.name.Type = %q, want username_t (per local override)", a.Type)
			}
		}
	}
}
