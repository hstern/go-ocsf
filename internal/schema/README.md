# OCSF schema — vendored

Each `v<major>.<minor>.<patch>/` subdirectory holds a verbatim
copy of the upstream [OCSF schema][upstream] at the matching
release tag. The codegen pipeline in `internal/gen/` consumes
exactly one of these directories per library release; the
`ocsf.SchemaVersion` constant in the root package is the single
source of truth for which one.

[upstream]: https://github.com/ocsf/ocsf-schema

## Vendoring policy

- **Schema is vendored, not fetched at build time.** Reproducible
  builds beat freshness. A schema-version bump is an explicit
  pull request that updates the vendored copy and re-runs codegen
  in the same commit (or commit pair, when the codegen volume
  warrants it).
- **One directory per schema version.** New schema versions land
  as a new sibling directory (`v1.4.0/` next to `v1.3.0/`), and the
  package-level `ocsf.SchemaVersion` constant switches to the new
  one. Old directories may be retained for one release cycle to
  ease consumer migration, then removed.
- **Each vendored directory carries an `UPSTREAM` file** recording
  the upstream tag and commit SHA, plus a short note of which
  non-schema artifacts (READMEs, CI configs, image assets) were
  dropped. The verification recipe in `UPSTREAM` lets a reviewer
  diff the vendored tree against the upstream tag in one command.
- **No local edits to vendored content.** If a workaround is
  needed (e.g. the upstream JSON has a typo blocking codegen),
  fix it in the codegen tool rather than patching the vendored
  schema. Patches to vendored content are reviewed by a reviewer
  in a separate PR with the patch's rationale spelled out in the
  body.
- **License attribution stays with the vendored content.** The
  upstream `LICENSE` and `NOTICE` files travel with each vendored
  directory; the library's own root-level `LICENSE` covers the
  hand-written code and the codegen output, not the schema JSON
  it consumes.

## Why this is `internal/`

Consumers of `go-ocsf` use the generated types in `events/` and
`objects/` plus the hand-written core in the root `ocsf` package.
The schema JSON itself is a codegen input, not a consumer-visible
artifact, so the `internal/` placement prevents external code from
taking a load-bearing dependency on the schema's on-disk layout
(which is the upstream project's to change, not ours).

Code that needs the schema at runtime — for example, a future
runtime-validation feature that consults the dictionary — would
embed the relevant pieces via `//go:embed` in the package that
needs them, rather than re-exposing the `internal/schema/` tree.
