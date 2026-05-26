// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

package emit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hstern/go-ocsf/internal/gen/schema"
)

// TestEmit_SweepRemovesStaleGenerated verifies the load-bearing
// property OCSF-30 adds: after Emit runs over a schema whose
// class set is smaller than the previous run's, codegen output
// for the dropped classes is gone from disk.
//
// Constructs two single-event schemas with the same category but
// different class names, runs Emit against the first, then
// against the second, then asserts pass-one's file is gone and
// pass-two's is present.
func TestEmit_SweepRemovesStaleGenerated(t *testing.T) {
	dir := t.TempDir()

	pass1 := miniSchemaWithEvent("event_log", 1008)
	if _, err := Emit(Options{Schema: pass1, OutDir: dir}); err != nil {
		t.Fatalf("Emit pass1: %v", err)
	}
	pass1File := filepath.Join(dir, "events", "system", "event_log.go")
	if _, err := os.Stat(pass1File); err != nil {
		t.Fatalf("pass1 file not written: %v", err)
	}

	pass2 := miniSchemaWithEvent("event_log_actvity", 1008)
	r, err := Emit(Options{Schema: pass2, OutDir: dir})
	if err != nil {
		t.Fatalf("Emit pass2: %v", err)
	}

	if _, err := os.Stat(pass1File); !os.IsNotExist(err) {
		t.Errorf("pass1 file still on disk after pass2 sweep: stat err = %v", err)
	}
	pass2File := filepath.Join(dir, "events", "system", "event_log_actvity.go")
	if _, err := os.Stat(pass2File); err != nil {
		t.Errorf("pass2 file missing: %v", err)
	}
	if len(r.SweptFiles) != 1 || r.SweptFiles[0] != pass1File {
		t.Errorf("SweptFiles = %v, want [%s]", r.SweptFiles, pass1File)
	}
}

// TestEmit_SweepPreservesHandWritten asserts the defensive
// check on the codegen marker — a hand-written file under one
// of the generated directories is NOT touched by the sweep.
func TestEmit_SweepPreservesHandWritten(t *testing.T) {
	dir := t.TempDir()

	// Pre-populate a hand-written file under objects/ before the
	// first emit, so it's there alongside the codegen output.
	objDir := filepath.Join(dir, "objects")
	if err := os.MkdirAll(objDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	handWritten := filepath.Join(objDir, "my_helper.go")
	const handWrittenBody = "// Hand-written helper — not generated.\n\npackage objects\n\nfunc Helper() {}\n"
	if err := os.WriteFile(handWritten, []byte(handWrittenBody), 0o644); err != nil {
		t.Fatalf("write hand-written: %v", err)
	}

	// Emit a tiny schema (one event). The hand-written file is
	// not in the keep set; if the marker check is doing its job,
	// the sweep skips it.
	if _, err := Emit(Options{Schema: miniSchemaWithEvent("event_log", 1008), OutDir: dir}); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	if _, err := os.Stat(handWritten); err != nil {
		t.Errorf("hand-written file removed by sweep: %v", err)
	}
}

// TestEmit_SweepIsIdempotent confirms a fresh tree → emit →
// re-emit produces an empty SweptFiles on the second pass
// (everything written by pass 1 stays in the keep set on pass
// 2 because the schema is the same).
func TestEmit_SweepIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	s := miniSchemaWithEvent("event_log", 1008)
	if _, err := Emit(Options{Schema: s, OutDir: dir}); err != nil {
		t.Fatalf("Emit pass1: %v", err)
	}
	r, err := Emit(Options{Schema: s, OutDir: dir})
	if err != nil {
		t.Fatalf("Emit pass2: %v", err)
	}
	if len(r.SweptFiles) != 0 {
		t.Errorf("idempotent re-emit swept %v, want []", r.SweptFiles)
	}
}

// miniSchemaWithEvent builds a single-event schema sufficient
// for the sweep tests. The event lives in the system category
// (which gets class_uid 1xxx) so the emitter writes it to
// events/system/<name>.go.
func miniSchemaWithEvent(eventName string, classUID int) *schema.Schema {
	return &schema.Schema{
		Version: "test",
		Categories: map[string]schema.Category{
			"system": {Name: "system", Caption: "System Activity", UID: 1},
		},
		Dictionary: schema.Dictionary{
			Attributes: map[string]schema.DictAttr{},
			Types:      map[string]schema.TypeDef{"string_t": {Name: "string_t"}},
		},
		Objects: map[string]schema.ObjectClass{},
		Events: map[string]schema.EventClass{
			eventName: {
				Name:     eventName,
				Caption:  "Test Event",
				Category: "system",
				UID:      classUID - 1000, // 1008 → 8
			},
		},
	}
}
