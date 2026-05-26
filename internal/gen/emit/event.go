// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

package emit

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/hstern/go-ocsf/internal/gen/schema"
)

// eventPackageName returns the short Go package name for an
// event class's emitted file. Concrete classes go in their
// category's package (e.g. authentication -> "iam",
// detection_finding -> "findings"). The abstract `base_event`
// class has category "other" — a sentinel for "no category" —
// and lands in a sibling `base` package so the type isn't
// jammed into a category it doesn't belong in.
func eventPackageName(ec schema.EventClass) string {
	if ec.Category == "" || ec.Category == "other" {
		return "base"
	}
	return ec.Category
}

// eventPackageDir returns the on-disk path (relative to the
// output root) where eventPackageName's package files live.
func eventPackageDir(ec schema.EventClass) string {
	return "events/" + eventPackageName(ec)
}

// eventFileName returns the file basename for an event class.
// Same shape as object file names: drop leading underscores
// (no such names in events at 1.3.0, but stays consistent with
// objectFileName), append .go.
func eventFileName(ec schema.EventClass) string {
	return strings.TrimLeft(ec.Name, "_") + ".go"
}

// writeEventFile renders one OCSF event class as a Go source
// file. Each event class produces:
//
//   - the struct itself, with one field per resolved attribute;
//   - ClassUID() int, CategoryUID() int, ClassName() string
//     metadata accessors so consumers can identify an event
//     without re-parsing its bytes;
//   - a CategoryName() string accessor returning the OCSF
//     category name (the snake_case identifier from
//     categories.json).
//
// Abstract category-root classes (iam, network, finding,
// base_event) have UID == 0 and still get the methods: their
// ClassUID is CategoryUID*1000 + 0 (e.g. iam -> 3000), with
// base_event returning 0 across the board.
func writeEventFile(w io.Writer, s *schema.Schema, ec schema.EventClass) error {
	pkg := eventPackageName(ec)
	if err := writeFileHeader(w, pkg); err != nil {
		return err
	}
	imports := map[string]bool{}
	for _, a := range ec.Attributes { //nolint:gocritic // copy fine in codegen path
		for _, imp := range fieldImports(a, pkg) {
			imports[imp] = true
		}
	}
	// Concrete event classes (UID != 0) register themselves with
	// the root ocsf package's class registry via an init()
	// function, which requires an import of the root package.
	// Abstract category-root classes (iam, network, finding,
	// base_event) skip registration entirely — they have UID 0
	// and aren't on the wire as themselves.
	registers := s.ClassUID(ec) != 0
	if registers {
		imports[rootPkg] = true
	}
	// Validate() constructs *ocsf.ValidationError on each
	// required-field violation, so a class with any
	// non-skippable required field also imports the root
	// package. Abstract classes inherit required fields from
	// base_event and need this even when they don't register.
	if eventHasValidatableRequired(s, ec) {
		imports[rootPkg] = true
	}
	// Enum membership and class-level constraint checks also
	// construct *ocsf.ValidationError. Classes with any enum
	// attribute or any constraints block need the root import
	// even if they have no required field of a validatable
	// type.
	if eventHasEnumOrConstraints(ec) {
		imports[rootPkg] = true
	}
	if len(imports) > 0 {
		impPaths := make([]string, 0, len(imports))
		for p := range imports {
			impPaths = append(impPaths, p)
		}
		sort.Strings(impPaths)
		if _, err := fmt.Fprintln(w, "import ("); err != nil {
			return err
		}
		for _, p := range impPaths {
			if _, err := fmt.Fprintf(w, "\t%q\n", p); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w, ")"); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}

	typeName := goName(ec.Name)
	if err := writeEventGodoc(w, ec, typeName, s); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "type %s struct {\n", typeName); err != nil {
		return err
	}
	for i, a := range ec.Attributes { //nolint:gocritic // copy fine in codegen path
		if i > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		if err := writeField(w, s, a, pkg); err != nil {
			return fmt.Errorf("attribute %q: %w", a.Name, err)
		}
	}
	if _, err := fmt.Fprintln(w, "}"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if err := writeEventMethods(w, s, ec, typeName); err != nil {
		return err
	}
	if err := writeEventValidate(w, s, ec, typeName); err != nil {
		return err
	}
	if registers {
		if err := writeEventRegistration(w, s, ec, typeName); err != nil {
			return err
		}
	}
	return nil
}

// writeEventValidate emits a Validate() method that checks each
// required attribute for presence. The check varies by Go type:
//
//   - pointer fields (object references):     value == nil
//   - string fields:                          value == ""
//   - slice fields:                           len(value) == 0
//   - json.RawMessage (special-case slice):   len(value) == 0
//
// Numeric (int, int64, float64) and boolean fields are
// deliberately NOT checked. Go's encoding/json can't
// distinguish "field absent" from "field present with zero
// value" on these without a pointer wrapper, and Phase 2's
// design chose plain int/bool over *int/*bool to keep the
// wire-stable round-trip cheap. Required-but-zero is reported
// as success here; a future strict mode could lift this.
//
// The method has a value receiver to match the existing
// OCSF*-prefixed metadata accessors; consistency across a
// type's method set is the Go discipline. Validation doesn't
// mutate, so the value receiver imposes no semantic cost.
//
// Validate stops at the first violation and returns it,
// matching the [ocsf.ValidationError] doc's "first violation"
// rule. Exhaustive enumeration is a follow-up.
func writeEventValidate(w io.Writer, s *schema.Schema, ec schema.EventClass, typeName string) error {
	required, err := requiredFieldsForValidate(s, ec)
	if err != nil {
		return err
	}
	classUID := s.ClassUID(ec)
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "// Validate checks the required-field rules for %s.\n", typeName); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "// Returns the first violation found, or nil if all required fields are present."); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "func (e %s) Validate() error {\n", typeName); err != nil {
		return err
	}
	for _, rc := range required {
		check, ok := requiredFieldCheck(rc.goType)
		if !ok {
			continue
		}
		if _, err := fmt.Fprintf(w, "\tif %s {\n", fmt.Sprintf(check, "e."+rc.goField)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "\t\treturn &ocsf.ValidationError{ClassUID: %d, Field: %q, Rule: \"required\", Reason: \"required field is missing\"}\n", classUID, rc.ocsfName); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "\t}"); err != nil {
			return err
		}
	}
	if err := writeEnumValidate(w, nil, ec, classUID); err != nil {
		return err
	}
	if err := writeConstraintValidate(w, ec, classUID); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "\treturn nil"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "}"); err != nil {
		return err
	}
	return nil
}

// writeConstraintValidate emits checks for the class-level
// constraint groups upstream declares — `at_least_one` and
// `just_one`. Both predicates count how many of the listed
// fields are non-zero by the same rules as the required-field
// check (pointer != nil, slice non-empty, string non-empty,
// json.RawMessage non-empty); numeric and boolean fields can't
// participate by codegen-time choice (their zero value is
// indistinguishable from absence on the wire) and are excluded
// from the count.
//
// at_least_one: count must be >= 1.
// just_one:     count must be == 1.
//
// Per the OCSF-20 scope note ("capture only the constraints the
// schema explicitly calls out"), nothing here invents rules —
// at_least_one and just_one are the two constraint kinds the
// upstream metaschema defines in its `constraints` block.
// Cross-field rules that aren't expressed as constraints (e.g.
// status_id == 99 implies status_detail) stay out of scope
// until upstream codifies them.
//
// The check runs after required-field and enum checks so a
// fully-missing class surfaces as "missing required field"
// rather than "violated constraint" — the more actionable
// error.
func writeConstraintValidate(w io.Writer, ec schema.EventClass, classUID int) error {
	attrByName := map[string]schema.ClassAttr{}
	for _, a := range ec.Attributes { //nolint:gocritic // copy fine in codegen path
		attrByName[a.Name] = a
	}
	if len(ec.Constraints.AtLeastOne) > 0 {
		if err := writeAtLeastOneCheck(w, ec.Constraints.AtLeastOne, attrByName, classUID); err != nil {
			return err
		}
	}
	if len(ec.Constraints.JustOne) > 0 {
		if err := writeJustOneCheck(w, ec.Constraints.JustOne, attrByName, classUID); err != nil {
			return err
		}
	}
	return nil
}

// writeAtLeastOneCheck emits an OR-chain over the "field is
// present" expressions for each listed attribute. If none are
// present, returns a constraint ValidationError naming the
// joined attribute list (so the consumer knows what to look
// at).
func writeAtLeastOneCheck(w io.Writer, fields []string, attrByName map[string]schema.ClassAttr, classUID int) error {
	checks := constraintFieldExprs(fields, attrByName, "e.")
	if len(checks) == 0 {
		// Every attribute had a non-checkable type or wasn't on
		// the class. Skip emitting a tautological check.
		return nil
	}
	if _, err := fmt.Fprintf(w, "\tif !(%s) {\n", strings.Join(checks, " || ")); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "\t\treturn &ocsf.ValidationError{ClassUID: %d, Field: %q, Rule: \"constraint\", Reason: \"at_least_one: at least one of these fields must be set\"}\n", classUID, strings.Join(fields, ",")); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "\t}"); err != nil {
		return err
	}
	return nil
}

// writeJustOneCheck emits a count expression: convert each
// "field present" predicate to int via a tiny inline helper
// (1 if present, 0 otherwise), sum them, and assert == 1.
//
// Implementation note: Go has no direct bool→int conversion,
// so the generated code uses one-line ternary-style assignments
// (count := 0; if X { count++ } ...) inside an anonymous
// scope. The verbose form is what gofmt produces; cuter
// alternatives (map sums, reflect-based counts) would
// undermine the cheap "emitted code is obvious Go" goal.
func writeJustOneCheck(w io.Writer, fields []string, attrByName map[string]schema.ClassAttr, classUID int) error {
	checks := constraintFieldExprs(fields, attrByName, "e.")
	if len(checks) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(w, "\t{"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "\t\tcount := 0"); err != nil {
		return err
	}
	for _, c := range checks {
		if _, err := fmt.Fprintf(w, "\t\tif %s {\n\t\t\tcount++\n\t\t}\n", c); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w, "\t\tif count != 1 {"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "\t\t\treturn &ocsf.ValidationError{ClassUID: %d, Field: %q, Rule: \"constraint\", Reason: \"just_one: exactly one of these fields must be set\"}\n", classUID, strings.Join(fields, ",")); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "\t\t}"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "\t}"); err != nil {
		return err
	}
	return nil
}

// constraintFieldExprs returns the "field is present"
// expressions for the named fields, in input order, skipping
// fields whose Go type doesn't permit a defensible
// present-vs-absent check (numeric / bool, per the same
// reasoning as required-field emission) or that aren't on the
// class at all. The prefix is prepended to each field
// reference (typically "e." for the value receiver).
func constraintFieldExprs(fields []string, attrByName map[string]schema.ClassAttr, prefix string) []string {
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		attr, ok := attrByName[f]
		if !ok {
			continue
		}
		expr, ok := constraintPresentExpr(attr, prefix+goFieldName(attr.Name))
		if !ok {
			continue
		}
		out = append(out, expr)
	}
	return out
}

// constraintPresentExpr returns the Go boolean expression that
// is true when the given attribute is present on the wire (not
// the zero value of its Go type). Mirrors requiredFieldCheck's
// rules but emits the positive form ("is present") rather than
// the negative form ("is missing").
func constraintPresentExpr(a schema.ClassAttr, fieldRef string) (string, bool) {
	if a.IsArray {
		return "len(" + fieldRef + ") != 0", true
	}
	if p := primitiveGoType(a.Type); p != "" {
		switch p {
		case "string":
			return fieldRef + " != \"\"", true
		case "json.RawMessage":
			return "len(" + fieldRef + ") != 0", true
		}
		// int / int64 / float64 / bool — zero indistinguishable
		// from absent, skip.
		return "", false
	}
	// Object reference (non-primitive type) — pointer.
	return fieldRef + " != nil", true
}

// writeEnumValidate emits the enum-membership and
// sibling-correspondence checks per OCSF-17. The checks run
// after the required-field checks so a required-but-zero enum
// field surfaces as "missing" (when its Go type allows that
// detection) rather than "out-of-enum"; the inverse error
// message would be misleading.
//
// Membership: for every class attribute whose Go type is `int`
// and whose resolved Enum is non-empty, emit a switch that
// covers the known values and returns
// `ValidationError{Rule: "enum"}` from the default arm.
//
// Sibling correspondence: for every `<x>_id` attribute that has
// a sibling string field present on the same class, emit a
// switch that asserts the sibling string equals the upstream
// caption of the matching enum value. The OCSF "Other"
// convention — typically id 99 with caption "Other" — is
// the documented escape valve for free-form sibling strings,
// so the correspondence check skips when the id is the "Other"
// value.
//
// The emitter walks attributes in name-sorted order so the
// generated switch arms come out byte-stable across runs (the
// codegen-diff gate from OCSF-12 depends on this).
func writeEnumValidate(w io.Writer, _ *schema.Schema, ec schema.EventClass, classUID int) error {
	enumAttrs := enumAttrsForValidate(ec)
	attrByName := map[string]schema.ClassAttr{}
	for _, a := range ec.Attributes { //nolint:gocritic // copy fine in codegen path
		attrByName[a.Name] = a
	}
	for _, a := range enumAttrs { //nolint:gocritic // copy fine in codegen path
		if err := writeEnumMembershipCheck(w, a, classUID); err != nil {
			return err
		}
		if a.Sibling == "" {
			continue
		}
		sib, ok := attrByName[a.Sibling]
		if !ok || sib.Type != "string_t" || sib.IsArray {
			// Sibling not on this class, or has an
			// unexpected shape — skip the correspondence check
			// rather than emit code that won't compile.
			continue
		}
		if err := writeSiblingCorrespondenceCheck(w, a, sib, classUID); err != nil {
			return err
		}
	}
	return nil
}

// enumAttrsForValidate returns the class's enum-bearing scalar
// int attributes in name-sorted order. Non-int enums
// (e.g. the `depth` string enum from OCSF-11) are skipped;
// they have no out-of-set check we can express against a
// `string` field, and upstream's "Other" sibling-escape
// convention doesn't apply.
func enumAttrsForValidate(ec schema.EventClass) []schema.ClassAttr {
	out := make([]schema.ClassAttr, 0, len(ec.Attributes))
	for _, a := range ec.Attributes { //nolint:gocritic // copy fine in codegen path
		if len(a.Enum) == 0 {
			continue
		}
		if a.IsArray {
			continue
		}
		if a.Type != "integer_t" && a.Type != "long_t" {
			continue
		}
		out = append(out, a)
	}
	return out
}

// writeEnumMembershipCheck emits the switch-based
// out-of-set check for a single enum attribute. The arms
// cover every value present in the resolved Enum (dictionary +
// any per-class additions); the default arm returns
// ValidationError{Rule:"enum"}.
func writeEnumMembershipCheck(w io.Writer, a schema.ClassAttr, classUID int) error {
	ids := sortedEnumIDsInt(a.Enum)
	fieldName := goFieldName(a.Name)
	if _, err := fmt.Fprintf(w, "\tswitch e.%s {\n", fieldName); err != nil {
		return err
	}
	if _, err := fmt.Fprint(w, "\tcase "); err != nil {
		return err
	}
	for i, id := range ids {
		if i > 0 {
			if _, err := fmt.Fprint(w, ", "); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprint(w, id); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w, ":"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "\tdefault:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "\t\treturn &ocsf.ValidationError{ClassUID: %d, Field: %q, Rule: \"enum\", Reason: \"value outside the schema's enum range\"}\n", classUID, a.Name); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "\t}"); err != nil {
		return err
	}
	return nil
}

// writeSiblingCorrespondenceCheck emits the per-value
// correspondence assertion linking an `<x>_id` attribute to
// its sibling string. The OCSF "Other" convention (typically
// id == 99 with caption "Other") is the documented escape
// valve for free-form sibling strings — we skip that arm so
// publishers using it for vendor-specific labels don't trip
// the check.
//
// The emitted code only fires when both the id is a known
// value (other than the Other escape) AND the sibling string
// is non-empty; an empty sibling is normal for a publisher
// that knows only the id.
func writeSiblingCorrespondenceCheck(w io.Writer, a, sib schema.ClassAttr, classUID int) error {
	idField := goFieldName(a.Name)
	sibField := goFieldName(sib.Name)
	otherID := otherEnumID(a.Enum)
	ids := sortedEnumIDsInt(a.Enum)
	if _, err := fmt.Fprintf(w, "\tif e.%s != \"\" {\n", sibField); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "\t\tswitch e.%s {\n", idField); err != nil {
		return err
	}
	for _, id := range ids {
		v := a.Enum[id]
		if otherID != "" && id == otherID {
			// Skip — the "Other" id allows any sibling
			// string by upstream convention.
			continue
		}
		caption := v.Caption
		if caption == "" {
			continue
		}
		if _, err := fmt.Fprintf(w, "\t\tcase %s:\n", id); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "\t\t\tif e.%s != %q {\n", sibField, caption); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "\t\t\t\treturn &ocsf.ValidationError{ClassUID: %d, Field: %q, Rule: \"enum\", Reason: \"sibling does not match enum caption\"}\n", classUID, sib.Name); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "\t\t\t}"); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w, "\t\t}"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "\t}"); err != nil {
		return err
	}
	return nil
}

// otherEnumID returns the enum key whose caption is "Other",
// if any. OCSF's convention is to assign id 99 to the
// free-form escape value, but the emitter detects by caption
// to stay robust against the rare case where upstream uses a
// different id.
func otherEnumID(enum map[string]schema.EnumValue) string {
	for id, v := range enum {
		if v.Caption == "Other" {
			return id
		}
	}
	return ""
}

// sortedEnumIDsInt returns the enum keys in ascending integer
// order. Non-numeric keys (none expected for int enums) are
// excluded; an enum whose keys aren't all numeric is treated
// as empty here, since the membership check needs numeric
// case labels.
func sortedEnumIDsInt(enum map[string]schema.EnumValue) []string {
	out := make([]string, 0, len(enum))
	for id := range enum {
		if _, err := strconv.Atoi(id); err != nil {
			continue
		}
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool {
		ai, _ := strconv.Atoi(out[i])
		bi, _ := strconv.Atoi(out[j])
		return ai < bi
	})
	return out
}

// eventHasEnumOrConstraints reports whether ec has at least
// one enum-bearing attribute or any class-level constraint
// group. Either one triggers an import of the root ocsf
// package because the emitted Validate body constructs
// *ocsf.ValidationError on violation.
func eventHasEnumOrConstraints(ec schema.EventClass) bool {
	if len(ec.Constraints.AtLeastOne) > 0 || len(ec.Constraints.JustOne) > 0 {
		return true
	}
	return len(enumAttrsForValidate(ec)) > 0
}

// eventHasValidatableRequired reports whether ec has at least
// one required attribute whose Go type is something Validate
// can meaningfully check (pointer, slice, string,
// json.RawMessage). Used to decide whether to import the root
// ocsf package on a class that only has required numeric/bool
// fields (no validatable check, no import needed).
func eventHasValidatableRequired(s *schema.Schema, ec schema.EventClass) bool {
	required, err := requiredFieldsForValidate(s, ec)
	if err != nil {
		return true // conservative — let the emitter surface the error
	}
	for _, rc := range required { //nolint:gocritic // copy fine in codegen path
		if _, ok := requiredFieldCheck(rc.goType); ok {
			return true
		}
	}
	return false
}

// requiredCheck describes one required-attribute slot at
// codegen time.
type requiredCheck struct {
	ocsfName string // wire-format snake_case identifier
	goField  string // Go struct field identifier
	goType   string // Go type expression
}

// requiredFieldsForValidate returns the required attributes on
// ec in attribute-name-sorted order, with their resolved Go
// field name and Go type. Used by writeEventValidate to emit
// the per-required-field checks.
func requiredFieldsForValidate(s *schema.Schema, ec schema.EventClass) ([]requiredCheck, error) {
	pkg := eventPackageName(ec)
	out := make([]requiredCheck, 0, len(ec.Attributes))
	for _, a := range ec.Attributes { //nolint:gocritic // copy fine in codegen path
		if a.Requirement != "required" {
			continue
		}
		typ, err := fieldGoType(s, a, pkg)
		if err != nil {
			return nil, fmt.Errorf("required attribute %q: %w", a.Name, err)
		}
		out = append(out, requiredCheck{
			ocsfName: a.Name,
			goField:  goFieldName(a.Name),
			goType:   typ,
		})
	}
	return out, nil
}

// requiredFieldCheck returns a fmt.Sprintf format string for
// the "is missing" predicate appropriate to the given Go type
// expression. The format string contains one %s placeholder
// for the field access (e.g. "e.User"). Returns ok=false for
// numeric and boolean types whose zero value is
// indistinguishable from absence — those fields are skipped at
// validate time.
func requiredFieldCheck(goType string) (string, bool) {
	switch {
	case strings.HasPrefix(goType, "*"):
		return "%s == nil", true
	case strings.HasPrefix(goType, "[]"):
		return "len(%s) == 0", true
	case goType == "string":
		return "%s == \"\"", true
	case goType == "json.RawMessage":
		return "len(%s) == 0", true
	}
	return "", false
}

// writeEventRegistration emits the init() function that puts a
// concrete event class into the root package's class registry.
// Required for [ocsf.Parse] to dispatch incoming payloads to
// the right typed event class without consumers explicitly
// importing each events/<category>/ subpackage by hand —
// importing any one of them side-effects the registration.
//
// The factory returns a pointer to a freshly-zeroed struct
// because json.Unmarshal needs an addressable target. Pointer
// satisfies Event automatically since the value-receiver
// methods promote to *T's method set.
func writeEventRegistration(w io.Writer, s *schema.Schema, ec schema.EventClass, typeName string) error {
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "func init() {\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "\tocsf.RegisterClass(%d, func() ocsf.Event { return &%s{} })\n", s.ClassUID(ec), typeName); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "}"); err != nil {
		return err
	}
	return nil
}

// writeEventGodoc emits the godoc paragraph above the struct
// declaration. Reproduces caption + description, surfaces the
// OCSF snake_case name and class_uid for cross-referencing, and
// emits Deprecated when upstream marks the class as such.
func writeEventGodoc(w io.Writer, ec schema.EventClass, typeName string, s *schema.Schema) error {
	desc := strings.TrimSpace(ec.Description)
	if desc == "" {
		desc = ec.Caption
	}
	if desc == "" {
		desc = "is an OCSF " + ec.Name + " event class."
	} else {
		desc = "describes the OCSF " + ec.Caption + " event class: " + desc
	}
	lines := wrapAndStripHTML(typeName+" "+desc, 70)
	for _, l := range lines {
		if _, err := fmt.Fprintf(w, "// %s\n", l); err != nil {
			return err
		}
	}
	uid := s.ClassUID(ec)
	if _, err := fmt.Fprintf(w, "//\n// OCSF name: %s. class_uid: %d.\n", ec.Name, uid); err != nil {
		return err
	}
	if ec.Deprecated != nil {
		if _, err := fmt.Fprintf(w, "//\n// Deprecated: %s\n", ec.Deprecated.Message); err != nil {
			return err
		}
	}
	return nil
}

// writeEventMethods emits the four metadata accessors —
// OCSFClassUID, OCSFCategoryUID, OCSFClassName, OCSFCategoryName.
// The naming differs from the build-plan's bare ClassUID /
// ClassName / CategoryUID because OCSF event classes also carry
// wire-format fields with those exact names (inherited from the
// classification include in base_event): a method named ClassUID
// would collide with the struct field ClassUID. Prefixing with
// OCSF disambiguates while keeping the canonical-constant
// accessors available for the Event interface (Phase 3).
//
// Each method returns a value resolved at codegen time, not at
// runtime — a consumer can switch on OCSFClassUID() without
// paying for a table lookup.
func writeEventMethods(w io.Writer, s *schema.Schema, ec schema.EventClass, typeName string) error {
	classUID := s.ClassUID(ec)
	className := ec.Caption
	if className == "" {
		className = ec.Name
	}
	categoryName := ec.Category
	categoryUID := 0
	if cat, ok := s.Categories[ec.Category]; ok {
		categoryUID = cat.UID
	}

	if _, err := fmt.Fprintf(w, "// OCSFClassUID returns the OCSF class_uid for %s (%d).\n", className, classUID); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "// Computed as CategoryUID*1000 + class identifier within the category."); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "func (%s) OCSFClassUID() int { return %d }\n\n", typeName, classUID); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "// OCSFClassName returns the OCSF class_name for %s.\n", className); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "func (%s) OCSFClassName() string { return %q }\n\n", typeName, className); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "// OCSFCategoryUID returns the OCSF category_uid for the %s category (%d).\n", categoryName, categoryUID); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "func (%s) OCSFCategoryUID() int { return %d }\n\n", typeName, categoryUID); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "// OCSFCategoryName returns the OCSF category_name (%s).\n", categoryName); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "func (%s) OCSFCategoryName() string { return %q }\n", typeName, categoryName); err != nil {
		return err
	}
	return nil
}
