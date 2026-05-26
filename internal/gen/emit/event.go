// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

package emit

import (
	"fmt"
	"io"
	"sort"
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
