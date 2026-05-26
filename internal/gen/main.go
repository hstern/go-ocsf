// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

// Command gen is the internal codegen tool for go-ocsf. It reads
// the vendored OCSF schema, builds an in-memory model (the
// internal/gen/schema package), and — in subsequent build phases —
// emits per-event-class and per-object Go types under events/ and
// objects/.
//
// At the OCSF-8 milestone the emission step is not yet wired up;
// running the tool simply loads the schema and prints a one-line
// summary, exercising the reader end-to-end against the vendored
// 1.3.0 tree.
//
// Invocation:
//
//	go run ./internal/gen -schema internal/schema/v1.3.0
//
// The schema flag is required so future schema-version bumps can
// pass a different vendored directory without code changes.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/hstern/go-ocsf/internal/gen/schema"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	schemaDir := fs.String("schema", "", "path to a vendored OCSF schema directory (e.g. internal/schema/v1.3.0)")
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
	return 0
}
