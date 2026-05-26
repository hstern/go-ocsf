// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

// Package specfixtures embeds the hand-curated OCSF
// sample-event payloads under v<schema-version>/ and exposes
// them via an io/fs.FS for the conformance tests in
// OCSF-23/24/25.
//
// The package is internal/ — the fixtures are test inputs,
// not a consumer-visible artifact. Consumers wanting
// canonical OCSF samples for their own tests should compose
// their own fixture set against their schema-version pin;
// the rationale is documented in
// internal/specfixtures/README.md.
package specfixtures

import "embed"

//go:embed v1.8.0
var v180 embed.FS

// V180 returns the embedded fixture tree for OCSF schema
// version 1.8.0. Sub-trees mirror events/<category>/ with
// `future/` carrying forward-compat fixtures whose class_uid
// the library intentionally doesn't recognize.
//
// The returned embed.FS is read-only and safe to share
// across goroutines; the conformance test walks it with
// fs.WalkDir.
func V180() embed.FS { return v180 }
