// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

// Command gen is the internal codegen tool for go-ocsf. It reads
// the vendored OCSF schema, builds an in-memory model (the
// internal/gen/schema package), and emits per-object and (in
// later phases) per-event-class Go types into the objects/ and
// events/ subpackages of the module.
//
// Usage:
//
//	# Summary mode — load schema and print a one-line summary.
//	go run ./internal/gen -schema internal/schema/v1.3.0
//
//	# Emit mode — load schema and write per-object Go files.
//	go run ./internal/gen -schema internal/schema/v1.3.0 -emit -out .
//
// The -schema flag is required so future schema-version bumps
// pass a different vendored directory without code changes. The
// -out flag, when -emit is set, is the repository-root path
// under which the emitter writes objects/<name>.go (and, in
// later phases, events/<category>/<name>.go).
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/hstern/go-ocsf/internal/gen/emit"
	"github.com/hstern/go-ocsf/internal/gen/schema"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	schemaDir := fs.String("schema", "", "path to a vendored OCSF schema directory (e.g. internal/schema/v1.3.0)")
	emitMode := fs.Bool("emit", false, "write Go source files for the loaded schema")
	outDir := fs.String("out", ".", "output directory when -emit is set (writes <out>/objects/*.go)")
	modulePath := fs.String("module", "github.com/hstern/go-ocsf", "Go module path of the repository being emitted into")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *schemaDir == "" {
		_, _ = fmt.Fprintln(stderr, "gen: -schema is required")
		fs.Usage()
		return 2
	}

	s, err := schema.Load(*schemaDir)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "gen: load %s: %v\n", *schemaDir, err)
		return 1
	}

	_, _ = fmt.Fprintf(stdout,
		"schema %s: %d categories, %d dictionary attributes, %d types, %d profiles, %d includes, %d objects, %d events\n",
		s.Version,
		len(s.Categories),
		len(s.Dictionary.Attributes),
		len(s.Dictionary.Types),
		len(s.Profiles),
		len(s.Includes),
		len(s.Objects),
		len(s.Events),
	)

	if !*emitMode {
		return 0
	}

	r, err := emit.Emit(emit.Options{
		Schema:     s,
		OutDir:     *outDir,
		ModulePath: *modulePath,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "gen: emit: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout,
		"emit: wrote %d object files under %s/objects/ and %d event files under %s/events/\n",
		len(r.ObjectFiles), *outDir, len(r.EventFiles), *outDir,
	)
	return 0
}
