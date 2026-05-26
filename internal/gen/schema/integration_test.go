// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

package schema_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/hstern/go-ocsf/internal/gen/schema"
)

// TestLoadVendoredV130 is the end-to-end smoke test: read the
// repository's vendored OCSF 1.3.0 schema and check the high-level
// counts plus a handful of spot-check invariants (well-known class
// UIDs, attribute resolution, profile presence). Asserting on
// counts catches the kind of partial-load failure where a single
// directory walk misses files; asserting on specific classes
// catches misresolved extends chains.
//
// Counts are reported by upstream's v1.3.0 release and verified by
// hand-counting the vendored directory; if upstream changes them
// in a future patch (unlikely for a tagged release), the test
// would need an update alongside the schema bump.
func TestLoadVendoredV130(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	schemaDir := filepath.Join(repoRoot, "internal", "schema", "v1.3.0")

	s, err := schema.Load(schemaDir)
	if err != nil {
		t.Fatalf("Load(%q): %v", schemaDir, err)
	}

	if got, want := s.Version, "1.3.0"; got != want {
		t.Errorf("Version = %q, want %q", got, want)
	}
	if got, want := len(s.Categories), 7; got != want {
		t.Errorf("Categories = %d, want %d (system, findings, iam, network, discovery, application, remediation)", got, want)
	}
	if c, found := s.Categories["iam"]; !found || c.UID != 3 {
		t.Errorf("Categories[iam] = %+v, want UID 3", c)
	}
	if got := len(s.Dictionary.Attributes); got < 500 {
		// Upstream has 627 attributes at 1.3.0; allow some slack
		// for benign upstream edits in a future patch but flag any
		// dramatic loss.
		t.Errorf("Dictionary.Attributes = %d, want >= 500", got)
	}
	if got := len(s.Dictionary.Types); got != 22 {
		t.Errorf("Dictionary.Types = %d, want 22", got)
	}
	if got := len(s.Objects); got < 100 {
		t.Errorf("Objects = %d, want >= 100", got)
	}
	if got := len(s.Events); got < 60 {
		t.Errorf("Events = %d, want >= 60", got)
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
	// dictionary type) and `email_addr` should remain whatever the
	// dictionary says.
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
