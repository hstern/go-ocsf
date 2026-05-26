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
		"file_path_t",
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

// objectsPkg is the Go-import path of the generated objects/
// package. Centralized here so a future module-path override
// flows through one constant rather than scattered strings.
const objectsPkg = "github.com/hstern/go-ocsf/objects"

// rootPkg is the Go-import path of the root ocsf package
// (where Event, BaseEvent, Parse, RegisterClass live). Used by
// concrete event-class emission to register themselves at
// init time.
const rootPkg = "github.com/hstern/go-ocsf"

// fieldImports lists every import path the package needs for a
// given attribute's Go type, given the currently-emitting Go
// package name. Returned in slice form to keep ordering
// deterministic at the call site (caller dedupes and sorts).
//
// currentPkg is the short Go package name being emitted (e.g.
// "objects", "iam", "network", "base"). When currentPkg ==
// "objects", an object-reference attribute doesn't need an
// import (the type is in scope). When emitting an event class
// in any other package, object references trigger an import of
// the objects package.
func fieldImports(a schema.ClassAttr, currentPkg string) []string {
	var out []string
	if primitiveGoType(a.Type) == "json.RawMessage" {
		out = append(out, "encoding/json")
	}
	if a.Type != "" && primitiveGoType(a.Type) == "" && currentPkg != "objects" {
		out = append(out, objectsPkg)
	}
	return out
}

// fieldGoType returns the Go-source-text representation of the
// attribute's wire-level type. currentPkg is the short Go
// package name being emitted; object references emitted from
// any package other than `objects` are qualified with
// `objects.`.
//
// Pointer-ness reflects two concerns: distinguishing absence
// from a wire-meaningful zero, and pairing with the
// `omitempty` JSON tag so optional-on-the-wire fields drop
// cleanly.
//
//   - object refs that are NOT array fields use *T so JSON
//     omitempty drops the field when absent. Array refs use
//     []T (slice nullability is the slice itself).
//   - non-required bool / int / int64 / float64 scalars use
//     *T so `false` and 0 round-trip as wire-meaningful values
//     rather than being squashed by omitempty. Required
//     numerics stay plain (always-present semantics; omitempty
//     irrelevant).
//   - string fields stay plain regardless of requirement: OCSF
//     practice treats empty-string as absence, so there's no
//     wire-meaningful "" to preserve.
//   - json.RawMessage stays a value (not pointer); a nil
//     RawMessage already omits.
func fieldGoType(s *schema.Schema, a schema.ClassAttr, currentPkg string) (string, error) {
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
		if shouldPointerWrap(a, p) {
			return "*" + p, nil
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
	if currentPkg != "objects" {
		tn = "objects." + tn
	}
	if a.IsArray {
		return "[]" + tn, nil
	}
	// Non-array object refs use *T so optional-with-omitempty
	// round-trips correctly: a nil pointer omits, a non-nil
	// pointer marshals the embedded struct.
	return "*" + tn, nil
}

// shouldPointerWrap reports whether a non-array primitive
// attribute should be emitted as *T rather than T. The two
// gates:
//
//   - The Go zero value must be wire-meaningful for the
//     attribute's type. true for bool (false is a real
//     answer), int / int64 / float64 (0 is a real
//     measurement, port, score, or id), and so on. false for
//     string (empty-string == absence in OCSF practice) and
//     json.RawMessage (nil-vs-empty already distinguishes).
//   - The attribute must NOT be required. Required attributes
//     always carry a value on the wire, so the absent-vs-zero
//     distinction is meaningless and the indirection
//     ergonomic cost is unjustified.
//
// goType is the resolved primitive Go type from
// primitiveGoType (e.g. "int", "bool", "string",
// "json.RawMessage"); pass it in rather than re-resolving so
// the caller's switch on type can stay simple.
func shouldPointerWrap(a schema.ClassAttr, goType string) bool {
	if a.Requirement == "required" {
		return false
	}
	switch goType {
	case "bool", "int", "int64", "float64":
		return true
	}
	return false
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
