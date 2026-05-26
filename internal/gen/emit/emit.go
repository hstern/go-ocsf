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
	"errors"
	"fmt"
	"go/format"
	"io"
	"io/fs"
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
	EventFiles  []string // sorted by file path
	EnumFiles   []string // sorted by file path
	SweptFiles  []string // codegen files removed because their class no longer exists in the schema
}

// Emit runs the codegen pass, writing one *.go file per OCSF
// object under opts.OutDir/objects/, per event class under
// opts.OutDir/events/<category>/, and per dictionary enum under
// opts.OutDir/enums/. Returns the list of files written so the
// caller can drive a codegen-diff comparison or surface counts
// in a CLI summary.
//
// After the write pass, Emit sweeps each of the three output
// directories and removes any `*.go` file that (a) starts with
// the codegen-emitted marker line and (b) wasn't written by
// the current run. This keeps the working tree honest when a
// schema bump removes or renames a class — without the sweep,
// the previous schema's file lingers and can trip init()
// duplicate-registration panics at runtime (precedent: the
// 1.3.0 → 1.8.0 cutover, where 1.3.0's events/system/event_log.go
// stayed alongside 1.8.0's events/system/event_log_actvity.go,
// both registering class_uid 1008).
//
// Hand-written files (no codegen marker on the first line) are
// left untouched. Result.SweptFiles lists what was removed.
//
// Emit is idempotent on the current schema: re-running it over
// the same schema produces byte-identical output and an empty
// SweptFiles list.
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

	eventFiles, err := emitEvents(opts)
	if err != nil {
		return nil, err
	}
	enumFiles, err := emitEnums(opts)
	if err != nil {
		return nil, err
	}
	r := &Result{ObjectFiles: written, EventFiles: eventFiles, EnumFiles: enumFiles}

	keep := map[string]bool{}
	for _, p := range r.ObjectFiles {
		keep[p] = true
	}
	for _, p := range r.EventFiles {
		keep[p] = true
	}
	for _, p := range r.EnumFiles {
		keep[p] = true
	}

	swept, err := sweepStale(opts.OutDir, keep)
	if err != nil {
		return nil, fmt.Errorf("emit: sweep: %w", err)
	}
	r.SweptFiles = swept
	return r, nil
}

// codegenMarker is the first source line every emitted file
// carries. sweepStale uses it to distinguish codegen output
// (safe to delete on re-run) from hand-written files that
// happen to live in the same tree (must NOT be deleted).
//
// Kept as a byte slice so we can compare with bytes.HasPrefix
// against the read-once header without converting to string.
var codegenMarker = []byte("// Code generated by internal/gen; DO NOT EDIT.")

// sweepStale walks the generated output directories under root
// and removes any *.go file that (a) carries the codegenMarker
// on its first line and (b) is not in the keep set (i.e. wasn't
// written by the current Emit run). Returns the removed paths
// in sort order for the Result.
//
// The defensive marker check is what makes the sweep safe
// against accidentally-clobbering a consumer's hand-written
// helper file under, say, events/iam/ — without the marker,
// the file isn't ours to delete.
func sweepStale(root string, keep map[string]bool) ([]string, error) {
	var removed []string
	roots := []string{
		filepath.Join(root, "objects"),
		filepath.Join(root, "enums"),
		filepath.Join(root, "events"),
	}
	for _, r := range roots {
		err := filepath.WalkDir(r, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					return nil // tree absent — nothing to sweep
				}
				return err
			}
			if d.IsDir() || !strings.HasSuffix(p, ".go") {
				return nil
			}
			if keep[p] {
				return nil
			}
			generated, err := isCodegenOutput(p)
			if err != nil {
				return fmt.Errorf("read %s: %w", p, err)
			}
			if !generated {
				return nil
			}
			if err := os.Remove(p); err != nil {
				return fmt.Errorf("remove %s: %w", p, err)
			}
			removed = append(removed, p)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(removed)
	return removed, nil
}

// isCodegenOutput reports whether the file at path was emitted
// by this package's codegen — i.e. its first non-empty line is
// the codegenMarker. The comparison reads only enough bytes to
// match the marker, avoiding a full read on every sweep.
func isCodegenOutput(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()
	buf := make([]byte, len(codegenMarker))
	n, err := io.ReadFull(f, buf)
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		return false, nil // file shorter than the marker — not ours
	}
	if err != nil {
		return false, err
	}
	return n == len(codegenMarker) && bytes.Equal(buf, codegenMarker), nil
}

// emitEvents writes one Go file per OCSF event class under
// opts.OutDir/events/<package>/<name>.go. Each category gets its
// own subpackage; the abstract base_event class lands in a
// sibling `base` package.
func emitEvents(opts Options) ([]string, error) {
	events := sortedEvents(opts.Schema.Events)
	var written []string
	createdDirs := map[string]bool{}
	for _, name := range events {
		ec := opts.Schema.Events[name]
		dir := filepath.Join(opts.OutDir, eventPackageDir(ec))
		if !createdDirs[dir] {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("emit: mkdir %s: %w", dir, err)
			}
			createdDirs[dir] = true
		}
		buf := &bytes.Buffer{}
		if err := writeEventFile(buf, opts.Schema, ec); err != nil {
			return nil, fmt.Errorf("emit event %q: %w", name, err)
		}
		formatted, err := format.Source(buf.Bytes())
		if err != nil {
			return nil, fmt.Errorf("emit event %q: format: %w\n--- pre-format ---\n%s", name, err, buf.String())
		}
		out := filepath.Join(dir, eventFileName(ec))
		if err := writeIfChanged(out, formatted); err != nil {
			return nil, fmt.Errorf("emit event %q: write: %w", name, err)
		}
		written = append(written, out)
	}
	sort.Strings(written)
	return written, nil
}

// sortedEvents returns the event names from m in name-sorted
// order — the canonical iteration order for deterministic
// output.
func sortedEvents(m map[string]schema.EventClass) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
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
