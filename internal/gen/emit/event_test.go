// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

package emit

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hstern/go-ocsf/internal/gen/schema"
)

func TestEventPackageName(t *testing.T) {
	cases := []struct {
		name string
		in   schema.EventClass
		want string
	}{
		{"concrete in iam", schema.EventClass{Name: "authentication", Category: "iam"}, "iam"},
		{"concrete in network", schema.EventClass{Name: "http_activity", Category: "network"}, "network"},
		{"abstract base_event", schema.EventClass{Name: "base_event", Category: "other"}, "base"},
		{"missing category", schema.EventClass{Name: "x", Category: ""}, "base"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := eventPackageName(c.in); got != c.want {
				t.Errorf("eventPackageName(%+v) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestEventPackageDir(t *testing.T) {
	if got, want := eventPackageDir(schema.EventClass{Category: "iam"}), "events/iam"; got != want {
		t.Errorf("eventPackageDir = %q, want %q", got, want)
	}
	if got, want := eventPackageDir(schema.EventClass{Category: "other"}), "events/base"; got != want {
		t.Errorf("eventPackageDir(other) = %q, want %q", got, want)
	}
}

func TestEventFileName(t *testing.T) {
	if got, want := eventFileName(schema.EventClass{Name: "authentication"}), "authentication.go"; got != want {
		t.Errorf("eventFileName = %q, want %q", got, want)
	}
}

func TestWriteEventFile_StructureMethodsAndImport(t *testing.T) {
	s := &schema.Schema{
		Categories: map[string]schema.Category{
			"iam": {Name: "iam", UID: 3},
		},
		Objects: map[string]schema.ObjectClass{
			"user": {Name: "user"},
		},
	}
	ec := schema.EventClass{
		Name:        "authentication",
		Caption:     "Authentication",
		Description: "Authn events.",
		Category:    "iam",
		UID:         2,
		Attributes: []schema.ClassAttr{
			{Name: "user", Type: "user", Requirement: "required"},
			{Name: "raw", Type: "json_t", Requirement: "optional"},
		},
	}

	buf := &bytes.Buffer{}
	if err := writeEventFile(buf, s, ec); err != nil {
		t.Fatalf("writeEventFile: %v", err)
	}
	out := buf.String()
	wants := []string{
		"package iam",
		`"encoding/json"`,
		`"github.com/hstern/go-ocsf/objects"`,
		"type Authentication struct",
		"User *objects.User `json:\"user\"`",
		"Raw json.RawMessage `json:\"raw,omitempty\"`",
		"func (Authentication) OCSFClassUID() int { return 3002 }",
		`func (Authentication) OCSFClassName() string { return "Authentication" }`,
		"func (Authentication) OCSFCategoryUID() int { return 3 }",
		`func (Authentication) OCSFCategoryName() string { return "iam" }`,
	}
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("writeEventFile output missing %q\n--- output ---\n%s", w, out)
		}
	}
}

func TestWriteEventFile_AbstractClassReturnsZeroClassUID(t *testing.T) {
	s := &schema.Schema{
		Categories: map[string]schema.Category{"iam": {Name: "iam", UID: 3}},
	}
	// Category-root abstract class: uid == 0. Concrete classes
	// extend it. ClassUID should be 0 (3 * 1000 + 0).
	ec := schema.EventClass{
		Name:     "iam",
		Caption:  "Identity & Access Management",
		Category: "iam",
		UID:      0,
	}
	buf := &bytes.Buffer{}
	if err := writeEventFile(buf, s, ec); err != nil {
		t.Fatalf("writeEventFile: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "func (IAM) OCSFClassUID() int { return 0 }") {
		t.Errorf("abstract iam class should have OCSFClassUID()=0\n--- output ---\n%s", out)
	}
}
