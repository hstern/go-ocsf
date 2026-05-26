// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"testing"
	"testing/fstest"
)

// synthetic builds a minimal in-memory OCSF schema covering every
// resolution rule the reader is responsible for: dictionary types,
// dictionary attributes, includes, profiles, extends (single + chain),
// $include, profile-null erasure, enum union, and category lookup.
// One fixture is reused by multiple tests so the shape stays in
// one place; if a test wants to vary a single field it shadows the
// relevant entry in the returned MapFS.
func synthetic() fstest.MapFS {
	return fstest.MapFS{
		"version.json": &fstest.MapFile{
			Data: []byte(`{"version": "9.9.9"}`),
		},
		"categories.json": &fstest.MapFile{
			Data: []byte(`{
				"caption": "Categories",
				"name": "category",
				"attributes": {
					"sample": {"caption": "Sample", "description": "Sample category", "uid": 7}
				}
			}`),
		},
		"dictionary.json": &fstest.MapFile{
			Data: []byte(`{
				"caption": "Dictionary",
				"description": "Test dictionary",
				"name": "dictionary",
				"attributes": {
					"id":     {"caption": "ID", "description": "test id", "type": "integer_t"},
					"action": {"caption": "Action", "description": "test action", "type": "string_t"},
					"action_id": {
						"caption": "Action ID", "description": "test action id",
						"type": "integer_t", "sibling": "action",
						"enum": {"1": {"caption": "Allow"}, "99": {"caption": "Other"}}
					},
					"name":   {"caption": "Name", "description": "test name", "type": "string_t"},
					"reason": {"caption": "Reason", "description": "test reason", "type": "string_t"},
					"emails": {"caption": "Emails", "description": "addresses", "type": "email_t", "is_array": true}
				},
				"types": {
					"caption": "Types",
					"attributes": {
						"string_t":  {"caption": "String", "description": "utf-8 string"},
						"integer_t": {"caption": "Integer", "description": "int"},
						"email_t":   {"caption": "Email", "description": "email", "type": "string_t", "type_name": "String", "max_len": 255}
					}
				}
			}`),
		},
		"includes/common.json": &fstest.MapFile{
			Data: []byte(`{
				"caption": "Common",
				"description": "Common attributes",
				"attributes": {
					"id":   {"requirement": "required", "group": "classification"},
					"name": {"requirement": "recommended", "group": "primary"}
				}
			}`),
		},
		"profiles/sample_profile.json": &fstest.MapFile{
			Data: []byte(`{
				"caption": "Sample Profile",
				"description": "Sample profile",
				"meta": "profile",
				"name": "sample_profile",
				"attributes": {
					"emails": {"requirement": "optional", "group": "context"}
				}
			}`),
		},
		"objects/base.json": &fstest.MapFile{
			Data: []byte(`{
				"caption": "Base",
				"description": "A base object",
				"name": "base",
				"extends": "object",
				"attributes": {
					"$include": ["includes/common.json"],
					"reason": {"requirement": "optional", "group": "context"}
				}
			}`),
		},
		"objects/leaf.json": &fstest.MapFile{
			Data: []byte(`{
				"caption": "Leaf",
				"description": "A leaf object that extends base",
				"name": "leaf",
				"extends": "base",
				"attributes": {
					"action_id": {
						"requirement": "required",
						"enum": {"2": {"caption": "Deny"}}
					}
				}
			}`),
		},
		"events/sample/example.json": &fstest.MapFile{
			Data: []byte(`{
				"caption": "Example",
				"description": "An example event class",
				"name": "example",
				"category": "sample",
				"uid": 1,
				"profiles": ["sample_profile"],
				"attributes": {
					"$include": ["includes/common.json", "profiles/sample_profile.json"],
					"reason": {"requirement": "optional", "group": "context"}
				}
			}`),
		},
		"events/sample/example_erased.json": &fstest.MapFile{
			Data: []byte(`{
				"caption": "Example Erased",
				"description": "Erases an inherited attribute via profile:null",
				"name": "example_erased",
				"category": "sample",
				"uid": 2,
				"extends": "example",
				"attributes": {
					"emails": null
				}
			}`),
		},
	}
}

func TestLoadFS_Synthetic(t *testing.T) {
	s, err := LoadFS(synthetic())
	if err != nil {
		t.Fatalf("LoadFS: %v", err)
	}
	if got, want := s.Version, "9.9.9"; got != want {
		t.Errorf("Version = %q, want %q", got, want)
	}
	if got, want := len(s.Categories), 1; got != want {
		t.Errorf("Categories = %d, want %d", got, want)
	}
	if c, ok := s.Categories["sample"]; !ok || c.UID != 7 {
		t.Errorf("Categories[sample] = %+v, want UID 7", c)
	}
	if got := len(s.Dictionary.Attributes); got != 6 {
		t.Errorf("Dictionary.Attributes = %d, want 6", got)
	}
	if got := len(s.Dictionary.Types); got != 3 {
		t.Errorf("Dictionary.Types = %d, want 3", got)
	}
	if got, want := len(s.Profiles), 1; got != want {
		t.Errorf("Profiles = %d, want %d", got, want)
	}
	// Includes contains both true includes/ files and profiles/ files
	// indexed by their resolution path: 1 + 1 = 2.
	if got, want := len(s.Includes), 2; got != want {
		t.Errorf("Includes = %d, want %d", got, want)
	}
	if got, want := len(s.Objects), 3; got != want {
		// base, leaf, plus the synthesized abstract "object" parent.
		t.Errorf("Objects = %d, want %d", got, want)
	}
	if got, want := len(s.Events), 2; got != want {
		t.Errorf("Events = %d, want %d", got, want)
	}
}

func TestResolve_IncludeMergesAttributes(t *testing.T) {
	s, err := LoadFS(synthetic())
	if err != nil {
		t.Fatalf("LoadFS: %v", err)
	}
	base, ok := s.Objects["base"]
	if !ok {
		t.Fatalf("Objects[base] missing")
	}
	// base $includes includes/common.json (id, name) and adds reason
	// locally. Expect three attributes after resolution.
	names := attrNames(base.Attributes)
	if want := []string{"id", "name", "reason"}; !equalStringSlices(names, want) {
		t.Errorf("base.Attributes names = %v, want %v", names, want)
	}
	// Per-class override merges with dictionary defaults: id has
	// type integer_t (dictionary) and requirement required ($include).
	for _, a := range base.Attributes {
		if a.Name == "id" {
			if a.Type != "integer_t" {
				t.Errorf("id.Type = %q, want integer_t", a.Type)
			}
			if a.Requirement != "required" {
				t.Errorf("id.Requirement = %q, want required", a.Requirement)
			}
			if a.Group != "classification" {
				t.Errorf("id.Group = %q, want classification", a.Group)
			}
		}
	}
}

func TestResolve_ExtendsInheritsAttributes(t *testing.T) {
	s, err := LoadFS(synthetic())
	if err != nil {
		t.Fatalf("LoadFS: %v", err)
	}
	leaf, ok := s.Objects["leaf"]
	if !ok {
		t.Fatalf("Objects[leaf] missing")
	}
	// leaf extends base (which has id, name, reason) and adds
	// action_id locally. Expect all four.
	names := attrNames(leaf.Attributes)
	if want := []string{"action_id", "id", "name", "reason"}; !equalStringSlices(names, want) {
		t.Errorf("leaf.Attributes names = %v, want %v", names, want)
	}
}

func TestResolve_EnumUnion(t *testing.T) {
	s, err := LoadFS(synthetic())
	if err != nil {
		t.Fatalf("LoadFS: %v", err)
	}
	leaf := s.Objects["leaf"]
	for _, a := range leaf.Attributes {
		if a.Name != "action_id" {
			continue
		}
		// Dictionary has 1=Allow and 99=Other; leaf adds 2=Deny.
		// Expect the union of three values.
		if got := len(a.Enum); got != 3 {
			t.Errorf("action_id.Enum size = %d, want 3 (got %v)", got, a.Enum)
		}
		if v, ok := a.Enum["2"]; !ok || v.Caption != "Deny" {
			t.Errorf("action_id.Enum[2] = %+v, want caption Deny", v)
		}
		if _, ok := a.Enum["1"]; !ok {
			t.Errorf("action_id.Enum[1] from dictionary missing after merge")
		}
		return
	}
	t.Fatal("leaf.action_id not found in resolved attributes")
}

func TestResolve_ProfileIncludeAndCategory(t *testing.T) {
	s, err := LoadFS(synthetic())
	if err != nil {
		t.Fatalf("LoadFS: %v", err)
	}
	ex, ok := s.Events["example"]
	if !ok {
		t.Fatalf("Events[example] missing")
	}
	// example $includes includes/common.json (id, name) and
	// profiles/sample_profile.json (emails), plus local reason.
	names := attrNames(ex.Attributes)
	if want := []string{"emails", "id", "name", "reason"}; !equalStringSlices(names, want) {
		t.Errorf("example.Attributes names = %v, want %v", names, want)
	}
	// emails is array on the wire (per dictionary is_array=true) and
	// its type is email_t.
	for _, a := range ex.Attributes {
		if a.Name == "emails" {
			if !a.IsArray {
				t.Errorf("emails.IsArray = false, want true")
			}
			if a.Type != "email_t" {
				t.Errorf("emails.Type = %q, want email_t", a.Type)
			}
		}
	}
	if got, want := s.ClassUID(ex), 7001; got != want {
		t.Errorf("ClassUID(example) = %d, want %d", got, want)
	}
}

func TestResolve_ProfileNullErasesAttribute(t *testing.T) {
	s, err := LoadFS(synthetic())
	if err != nil {
		t.Fatalf("LoadFS: %v", err)
	}
	ex, ok := s.Events["example_erased"]
	if !ok {
		t.Fatalf("Events[example_erased] missing")
	}
	// example_erased extends example (which has emails after profile
	// include) and erases emails via `null`. Expect emails absent.
	for _, a := range ex.Attributes {
		if a.Name == "emails" {
			t.Errorf("example_erased.Attributes still includes emails after null erasure: %+v", a)
		}
	}
}

func TestLoadFS_RejectsExtendsCycle(t *testing.T) {
	m := fstest.MapFS{
		"version.json":    &fstest.MapFile{Data: []byte(`{"version": "1"}`)},
		"categories.json": &fstest.MapFile{Data: []byte(`{"attributes": {}}`)},
		"dictionary.json": &fstest.MapFile{Data: []byte(`{
			"caption": "d", "description": "d", "name": "d",
			"attributes": {}, "types": {"caption": "t", "attributes": {}}
		}`)},
		"objects/a.json": &fstest.MapFile{Data: []byte(`{
			"caption": "A", "description": "A", "name": "a", "extends": "b",
			"attributes": {}
		}`)},
		"objects/b.json": &fstest.MapFile{Data: []byte(`{
			"caption": "B", "description": "B", "name": "b", "extends": "a",
			"attributes": {}
		}`)},
	}
	if _, err := LoadFS(m); err == nil {
		t.Fatal("LoadFS: expected error on extends cycle, got nil")
	}
}

func TestLoadFS_RejectsUnknownInclude(t *testing.T) {
	m := fstest.MapFS{
		"version.json":    &fstest.MapFile{Data: []byte(`{"version": "1"}`)},
		"categories.json": &fstest.MapFile{Data: []byte(`{"attributes": {}}`)},
		"dictionary.json": &fstest.MapFile{Data: []byte(`{
			"caption": "d", "description": "d", "name": "d",
			"attributes": {}, "types": {"caption": "t", "attributes": {}}
		}`)},
		"objects/a.json": &fstest.MapFile{Data: []byte(`{
			"caption": "A", "description": "A", "name": "a", "extends": "object",
			"attributes": {"$include": ["includes/missing.json"]}
		}`)},
	}
	if _, err := LoadFS(m); err == nil {
		t.Fatal("LoadFS: expected error on unknown $include path, got nil")
	}
}

func TestLoadFS_RejectsUnknownExtends(t *testing.T) {
	m := fstest.MapFS{
		"version.json":    &fstest.MapFile{Data: []byte(`{"version": "1"}`)},
		"categories.json": &fstest.MapFile{Data: []byte(`{"attributes": {}}`)},
		"dictionary.json": &fstest.MapFile{Data: []byte(`{
			"caption": "d", "description": "d", "name": "d",
			"attributes": {}, "types": {"caption": "t", "attributes": {}}
		}`)},
		"objects/a.json": &fstest.MapFile{Data: []byte(`{
			"caption": "A", "description": "A", "name": "a", "extends": "ghost",
			"attributes": {}
		}`)},
	}
	if _, err := LoadFS(m); err == nil {
		t.Fatal("LoadFS: expected error on unknown extends target, got nil")
	}
}

func TestClassUID_AbstractAndOther(t *testing.T) {
	s := &Schema{
		Categories: map[string]Category{
			"iam":   {Name: "iam", UID: 3},
			"other": {Name: "other", UID: 0},
		},
	}
	// Concrete: 3 * 1000 + 2 = 3002.
	if got := s.ClassUID(EventClass{Name: "authn", Category: "iam", UID: 2}); got != 3002 {
		t.Errorf("ClassUID(iam/authn uid=2) = %d, want 3002", got)
	}
	// Abstract (UID == 0): zero.
	if got := s.ClassUID(EventClass{Name: "iam", Category: "iam", UID: 0}); got != 0 {
		t.Errorf("ClassUID abstract iam = %d, want 0", got)
	}
	// Unknown category: zero.
	if got := s.ClassUID(EventClass{Name: "x", Category: "unknown", UID: 5}); got != 0 {
		t.Errorf("ClassUID unknown category = %d, want 0", got)
	}
}

// attrNames returns the sorted attribute Name list for comparison.
func attrNames(attrs []ClassAttr) []string {
	out := make([]string, 0, len(attrs))
	for _, a := range attrs {
		out = append(out, a.Name)
	}
	return out
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
