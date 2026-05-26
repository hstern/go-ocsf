// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

// Package specfixtures embeds the vendored OCSF sample-event
// payloads under v<schema-version>/ and exposes them via an
// io/fs.FS for the conformance tests in OCSF-23 and beyond.
//
// The package is internal/ — the fixtures are codegen and test
// inputs, not a consumer-visible artifact. Consumers wanting
// canonical OCSF samples for their own tests should compose
// their own fixture set against their schema-version pin; the
// schema-version pin and curation rationale documented in
// internal/specfixtures/README.md.
package specfixtures

import "embed"

//go:embed v1.3.0
var v130 embed.FS

// V130 returns the embedded fixture tree for OCSF schema
// version 1.3.0. Sub-trees mirror events/<category>/ with
// `future/` carrying forward-compat fixtures whose class_uid
// the library intentionally doesn't recognize.
//
// The returned embed.FS is read-only and safe to share across
// goroutines; the conformance test walks it with fs.WalkDir.
func V130() embed.FS { return v130 }
