# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

The library version is independent of the OCSF schema version it
implements. The current schema version tracked is exposed as the
`ocsf.SchemaVersion` constant; bumping that constant is a library-minor
release.

## [Unreleased]

## [0.1.0] - 2026-05-26

Initial release. Typed encoder / decoder for Open Cybersecurity Schema
Framework (OCSF) events, schema version `1.8.0`.

### Added

- Hand-written core in package `ocsf`: `SchemaVersion`, `Event` interface,
  `BaseEvent` forward-compat fallback, `Parse([]byte) (Event, error)`,
  `RegisterClass`, category enum, `ValidationError`, time helpers.
- Codegen pipeline (`internal/gen`) driven by the upstream OCSF schema
  vendored as a git submodule at `internal/schema/upstream`. Emits
  per-event-class structs under `events/<category>/`, per-object structs
  under `objects/`, typed enum constants under `enums/`. Generated
  symbols carry doc-comment provenance from the schema.
- `Validate()` per event class: required-field checks, enum-membership
  checks, id↔label correspondence when both fields are set, hand-written
  cross-field rules (e.g. `status_id = Other` requires `status_detail`).
- Conformance suite under `internal/conformance`: byte-stable round-trip
  on all vendored sample fixtures via canonical equality (decode +
  remarshal with `json.Number`), plus forward-compat `BaseEvent` and
  `unmapped` passthrough coverage.
- CI on GitHub Actions: `static`, `test`, `lint`, `codegen-diff` required
  for merge; separate non-blocking `govulncheck` job.
- README quickstarts for the emitter and consumer paths, runnable
  `Example` functions for `Parse` and the round-trip pattern.

### Notes

- Zero non-test dependencies. Standard library only.
- Module path is `github.com/hstern/go-ocsf`. Go 1.26+.
- Library SemVer is independent of the OCSF spec version; the spec
  version tracked is `ocsf.SchemaVersion`.

[Unreleased]: https://github.com/hstern/go-ocsf/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/hstern/go-ocsf/releases/tag/v0.1.0
