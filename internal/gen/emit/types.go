// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

package emit

import (
	"fmt"

	"github.com/hstern/go-ocsf/internal/gen/schema"
)

// primitiveGoType maps an OCSF primitive type name (the values
// living under dictionary.types, plus the special "object" alias)
// to its Go-language representation. The mapping is fixed at the
// library level — bumping a schema version doesn't change Go
// types, only which fields use them.
//
// Notes on choices:
//
//   - integer_t -> int. OCSF integer attributes don't carry a
//     declared bit width; int is the idiomatic Go signed integer
//     and accommodates every observed value. Code that needs a
//     specific size should cast at the call site.
//   - long_t -> int64. Explicitly 8-byte per the dictionary.
//   - timestamp_t -> int64. The wire format is milliseconds since
//     the Unix epoch; the int64 stays on the wire as-is rather
//     than converting to time.Time at unmarshal, per the design
//     decision to preserve byte-stability. A separate helper in
//     the root ocsf package (Phase 3) lifts to time.Time on
//     demand.
//   - float_t -> float64. Go's default precision for real numbers.
//   - boolean_t -> bool.
//   - json_t and the special "object" attribute type ->
//     json.RawMessage. Both denote an open-ended carrier; using
//     RawMessage preserves byte-stability for round-trip while
//     letting consumers re-decode at their leisure.
//   - All string-derived primitives (email_t, ip_t, url_t, ...) ->
//     string. The format constraints live on Validate (Phase 4),
//     not the type.
//
// Returns "" when name isn't a known primitive (caller should
// then look up an object type).
func primitiveGoType(name string) string {
	switch name {
	case "string_t",
		"bytestring_t",
		"datetime_t",
		"email_t",
		"file_hash_t",
		"file_name_t",
		"hostname_t",
		"ip_t",
		"mac_t",
		"process_name_t",
		"resource_uid_t",
		"subnet_t",
		"url_t",
		"username_t",
		"uuid_t":
		return "string"
	case "integer_t":
		return "int"
	case "long_t", "timestamp_t":
		return "int64"
	case "port_t":
		return "int"
	case "float_t":
		return "float64"
	case "boolean_t":
		return "bool"
	case "json_t", "object":
		return "json.RawMessage"
	}
	return ""
}

// fieldImports lists every import path the package needs for a
// given attribute's Go type. Returned in slice form to keep
// ordering deterministic at the call site (caller dedupes and
// sorts).
func fieldImports(a schema.ClassAttr) []string {
	if primitiveGoType(a.Type) == "json.RawMessage" {
		return []string{"encoding/json"}
	}
	return nil
}

// fieldGoType returns the Go-source-text representation of the
// attribute's wire-level type within an objects package (no
// `objects.` qualifier needed). Pointer-ness reflects the
// attribute's requirement and whether it's a struct kind:
//
//   - object refs that are NOT array fields and NOT required use
//     *T so JSON omitempty can drop the field when absent. Object
//     refs that ARE array fields use []T (slice nullability is
//     the slice itself).
//   - primitive scalars use their Go type directly, omitempty
//     handling falls out from Go's zero-value semantics on the
//     marshal side.
//   - json.RawMessage stays a value (not pointer); a nil
//     RawMessage already omits.
func fieldGoType(s *schema.Schema, a schema.ClassAttr) (string, error) {
	if a.Type == "" {
		// An attribute whose dictionary lookup failed at load
		// time. Treat as opaque JSON so the field still
		// round-trips; codegen-diff would surface the gap as a
		// reviewable change.
		if a.IsArray {
			return "[]json.RawMessage", nil
		}
		return "json.RawMessage", nil
	}
	if p := primitiveGoType(a.Type); p != "" {
		if a.IsArray {
			return "[]" + p, nil
		}
		return p, nil
	}
	// Object reference. The referenced object MUST exist in the
	// resolved schema; if it doesn't, that's a structural bug
	// rather than something the emitter should silently paper over.
	if _, ok := s.Objects[a.Type]; !ok {
		return "", fmt.Errorf("attribute %q references unknown object type %q", a.Name, a.Type)
	}
	tn := goName(a.Type)
	if a.IsArray {
		return "[]" + tn, nil
	}
	// Non-array object refs use *T so optional-with-omitempty
	// round-trips correctly: a nil pointer omits, a non-nil
	// pointer marshals the embedded struct.
	return "*" + tn, nil
}

// jsonTagOpts returns the contents of the `json:"..."` struct tag
// for an attribute. Required attributes get a bare tag (no
// omitempty); recommended and optional attributes get omitempty.
// Empty Requirement (which the resolver leaves blank for include
// attributes that don't override it) is treated as optional, the
// safer default for a lenient consumer-side decoder.
func jsonTagOpts(a schema.ClassAttr) string {
	if a.Requirement == "required" {
		return a.Name
	}
	return a.Name + ",omitempty"
}
