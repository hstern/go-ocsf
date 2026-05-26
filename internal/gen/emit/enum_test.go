// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

package emit

import (
	"bytes"
	"strings"
	"testing"

	"github.com/hstern/go-ocsf/internal/gen/schema"
)

func TestEnumTypeName(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"severity_id", "Severity"},
		{"activity_id", "Activity"},
		{"risk_level_id", "RiskLevel"},
		{"class_uid", "Class"},
		{"category_uid", "Category"},
		{"auth_protocol_id", "AuthProtocol"},
		{"depth", "Depth"}, // no suffix to strip
	}
	for _, c := range cases {
		if got := enumTypeName(c.in); got != c.want {
			t.Errorf("enumTypeName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSanitizeCaption(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Unknown", "Unknown"},
		{"In Progress", "InProgress"},
		{"Anti-Virus", "AntiVirus"},
		{"DNS/HTTP", "DNSHTTP"}, // see note below
		{"99", "99"},
		{"", ""},
	}
	for _, c := range cases {
		if got := sanitizeCaption(c.in); got != c.want {
			t.Errorf("sanitizeCaption(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestEnumConstName(t *testing.T) {
	cases := []struct {
		typeName, id, caption, want string
	}{
		{"Severity", "5", "Critical", "SeverityCritical"},
		{"Severity", "1", "Informational", "SeverityInformational"},
		{"Status", "2", "In Progress", "StatusInProgress"},
		{"Severity", "99", "", "SeverityValue99"}, // caption empty
		{"Severity", "0", "Unknown", "SeverityUnknown"},
	}
	for _, c := range cases {
		got := enumConstName(c.typeName, c.id, c.caption)
		if got != c.want {
			t.Errorf("enumConstName(%q,%q,%q) = %q, want %q", c.typeName, c.id, c.caption, got, c.want)
		}
	}
}

func TestWriteEnumType_NumericInt(t *testing.T) {
	attr := schema.DictAttr{
		Name:        "severity_id",
		Caption:     "Severity ID",
		Description: "The normalized severity.",
		Type:        "integer_t",
		Enum: map[string]schema.EnumValue{
			"0":  {ID: "0", Caption: "Unknown", Description: "Severity unknown."},
			"4":  {ID: "4", Caption: "High", Description: "Action required immediately."},
			"99": {ID: "99", Caption: "Other", Description: "Severity unmapped."},
		},
	}
	buf := &bytes.Buffer{}
	writeEnumHeader(buf, "enums", true)
	if err := writeEnumType(buf, attr); err != nil {
		t.Fatalf("writeEnumType: %v", err)
	}
	out := buf.String()
	wants := []string{
		`import "strconv"`,
		"type Severity int",
		"SeverityUnknown Severity = 0",
		"SeverityHigh Severity = 4",
		"SeverityOther Severity = 99",
		"func (v Severity) String() string",
		`case SeverityUnknown:`,
		`return "Unknown"`,
		"return strconv.Itoa(int(v))",
	}
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("output missing %q\n--- output ---\n%s", w, out)
		}
	}
}

func TestWriteEnumType_StringEnum(t *testing.T) {
	attr := schema.DictAttr{
		Name:    "depth",
		Caption: "CVSS Depth",
		Type:    "string_t",
		Enum: map[string]schema.EnumValue{
			"Base":          {ID: "Base", Caption: "Base"},
			"Temporal":      {ID: "Temporal", Caption: "Temporal"},
			"Environmental": {ID: "Environmental", Caption: "Environmental"},
		},
	}
	buf := &bytes.Buffer{}
	writeEnumHeader(buf, "enums", false) // string-typed: no strconv
	if err := writeEnumType(buf, attr); err != nil {
		t.Fatalf("writeEnumType: %v", err)
	}
	out := buf.String()
	wants := []string{
		"type Depth string",
		`DepthBase Depth = "Base"`,
		`DepthTemporal Depth = "Temporal"`,
		"return string(v)",
	}
	notWants := []string{
		`import "strconv"`,
		"strconv.Itoa",
	}
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("string enum output missing %q\n--- output ---\n%s", w, out)
		}
	}
	for _, w := range notWants {
		if strings.Contains(out, w) {
			t.Errorf("string enum output contains unwanted %q\n--- output ---\n%s", w, out)
		}
	}
}

func TestSortedEnumIDs(t *testing.T) {
	// Numeric ids sort ascending; non-numeric appended after.
	enum := map[string]schema.EnumValue{
		"99":  {},
		"0":   {},
		"5":   {},
		"foo": {},
		"bar": {},
	}
	got := sortedEnumIDs(enum)
	want := []string{"0", "5", "99", "bar", "foo"}
	if len(got) != len(want) {
		t.Fatalf("len mismatch got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("sortedEnumIDs[%d] = %q, want %q (full got %v)", i, got[i], want[i], got)
		}
	}
}
