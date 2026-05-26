// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

// Package emit turns a resolved OCSF [schema.Schema] into Go
// source files. The emitter writes one *.go per object (and, when
// extended in OCSF-10, one per event class plus one per category
// containing event-class enums), with deterministic output: the
// same input schema produces byte-identical files on every run,
// which is what the codegen-diff CI gate (OCSF-12) relies on.
//
// Determinism comes from three discipline points: maps are never
// iterated directly (callers iterate over sorted slices from
// [schema.Schema]), generated identifiers use a fixed
// acronym/casing table in identifiers.go, and the output is run
// through go/format before write so whitespace is normalized.
package emit

import (
	"bytes"
	"fmt"
	"go/format"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hstern/go-ocsf/internal/gen/schema"
)

// Options controls a single emit run.
type Options struct {
	// Schema is the resolved OCSF schema to emit Go types from.
	Schema *schema.Schema

	// OutDir is the repository root path under which the emitter
	// writes its outputs (currently objects/<name>.go).
	OutDir string

	// ModulePath is the Go module path so generated files can
	// generate cross-package imports of sibling subpackages when
	// future code emission introduces them (events importing
	// objects, etc.). Empty is permitted; cross-package imports
	// stay omitted in that case.
	ModulePath string
}

// Result describes what an [Emit] run wrote.
type Result struct {
	ObjectFiles []string // sorted by file path
}

// Emit runs the codegen pass, writing one *.go file per OCSF
// object under opts.OutDir/objects/. Returns the list of files
// written so the caller can drive a codegen-diff comparison or
// surface counts in a CLI summary.
//
// Emit is idempotent: re-running it over the same schema produces
// byte-identical output. Files written by a previous Emit run
// that are no longer present in the schema are NOT deleted —
// callers driving a codegen-diff check should remove stale files
// out of band (in practice, `git diff --exit-code` over the
// objects/ tree catches them).
func Emit(opts Options) (*Result, error) {
	if opts.Schema == nil {
		return nil, fmt.Errorf("emit: Options.Schema is nil")
	}
	if opts.OutDir == "" {
		return nil, fmt.Errorf("emit: Options.OutDir is empty")
	}
	objDir := filepath.Join(opts.OutDir, "objects")
	if err := os.MkdirAll(objDir, 0o755); err != nil {
		return nil, fmt.Errorf("emit: mkdir %s: %w", objDir, err)
	}

	objs := sortedObjects(opts.Schema.Objects)
	var written []string
	for _, name := range objs {
		obj := opts.Schema.Objects[name]
		if shouldSkipObject(obj) {
			continue
		}
		buf := &bytes.Buffer{}
		if err := writeObjectFile(buf, opts.Schema, obj); err != nil {
			return nil, fmt.Errorf("emit object %q: %w", name, err)
		}
		formatted, err := format.Source(buf.Bytes())
		if err != nil {
			// On a formatting failure write the unformatted bytes
			// alongside so the operator can diagnose; emission
			// itself fails.
			return nil, fmt.Errorf("emit object %q: format: %w\n--- pre-format ---\n%s", name, err, buf.String())
		}
		out := filepath.Join(objDir, objectFileName(name))
		if err := writeIfChanged(out, formatted); err != nil {
			return nil, fmt.Errorf("emit object %q: write: %w", name, err)
		}
		written = append(written, out)
	}
	sort.Strings(written)
	return &Result{ObjectFiles: written}, nil
}

// shouldSkipObject reports whether an object should be omitted
// from the emitted code surface. The empty abstract root
// `object` carries no attributes and is consumed by the resolver
// (as the synthesized parent of every leaf); emitting it as a Go
// struct would add a confusing zero-field type with no fields.
func shouldSkipObject(obj schema.ObjectClass) bool {
	return obj.Name == "object"
}

// objectFileName maps an OCSF object name (which may start with
// `_` to indicate UI-hidden status) to the corresponding Go file
// name. The leading underscore is dropped; Go files starting with
// `_` are ignored by the toolchain, and the underscore carries no
// privacy semantics for an emitted public type anyway.
func objectFileName(ocsfName string) string {
	return strings.TrimLeft(ocsfName, "_") + ".go"
}

// sortedObjects returns the object names from m in name-sorted
// order — the canonical iteration order for deterministic output.
func sortedObjects(m map[string]schema.ObjectClass) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// writeIfChanged writes data to path only if the file's current
// contents differ. Avoids touching mtime on files that haven't
// changed across emit runs — keeps `git status` quiet during
// codegen iteration and lets editors keep their file watchers
// settled.
func writeIfChanged(path string, data []byte) error {
	existing, err := os.ReadFile(path)
	if err == nil && bytes.Equal(existing, data) {
		return nil
	}
	return os.WriteFile(path, data, 0o644)
}

// fileHeader is the standard preamble for every generated Go
// source file: the codegen marker (recognized by go/build per
// the convention documented at
// https://pkg.go.dev/cmd/go#hdr-Generate_Go_files_by_processing_source),
// the SPDX block, then the package declaration. The codegen
// marker MUST be the first source line (the build tool's regex
// is anchored), so the SPDX block follows it rather than
// preceding it as on hand-written files.
func writeFileHeader(w io.Writer, pkg string) error {
	const header = `// Code generated by internal/gen; DO NOT EDIT.

// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

package %s

`
	_, err := fmt.Fprintf(w, header, pkg)
	return err
}
