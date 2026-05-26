# OCSF schema — submodule

The `upstream/` subdirectory is a git submodule pointing at
the canonical [`ocsf/ocsf-schema`][upstream] repository, with
its HEAD pinned to the commit corresponding to the upstream
tag named in `ocsf.SchemaVersion` (currently `1.8.0`). The
codegen pipeline in `internal/gen/` consumes this submodule
directly.

[upstream]: https://github.com/ocsf/ocsf-schema

## Why a submodule and not a vendored copy?

Until OCSF-29 the schema was a verbatim copy of the upstream
tree under `internal/schema/v1.3.0/`. A submodule pinned to a
tagged commit gives the same reproducibility guarantee as
that vendored copy without maintaining a parallel duplicate
of the upstream JSON every time a schema version ships. The
pin is the submodule's commit SHA, which is itself an
ironclad reference that can't drift the way a copied
directory could.

The trade-off:

- **Submodule (this approach)** — clone takes a moment longer
  (`git submodule update --init` once per checkout), but the
  upstream tree is the single source of truth and a schema
  bump is a one-line change to `.gitmodules`'s effective
  commit pin (plus codegen output, plus `ocsf.SchemaVersion`).
- **Vendored copy (the previous approach)** — every schema
  bump duplicates every upstream JSON file into the
  repository's working tree, doubling the disk footprint and
  adding a diff to review against an upstream that already
  carries an immutable tag.

## Working with the submodule

A fresh clone needs the submodule initialized once:

```
git clone https://github.com/hstern/go-ocsf
cd go-ocsf
git submodule update --init --recursive
```

CI (GitHub Actions) checks out submodules automatically via
`actions/checkout@v5`'s `submodules: recursive` option on all
four jobs (`static`, `test`, `lint`, `codegen-diff`) plus the
daily `vuln` workflow.

## Schema-version bumps

A library-minor release that tracks a new upstream schema
version (e.g. `1.8.0` → `1.9.0`) is one PR that:

1. Advances the submodule to the new upstream commit:
   ```
   cd internal/schema/upstream
   git fetch
   git checkout <upstream-tag>
   cd -
   git add internal/schema/upstream
   ```
2. Updates `ocsf.SchemaVersion` in the root package's
   `ocsf.go` to match.
3. Re-runs `go generate ./...` and commits the resulting
   diffs in `events/`, `objects/`, `enums/`.
4. Updates `internal/specfixtures/v<old>/` to
   `internal/specfixtures/v<new>/`, adjusting hand-curated
   fixtures for any class / attribute / required-field
   changes upstream made.
5. Verifies `go test ./internal/conformance/...` stays green.

The `codegen-diff` CI gate enforces step 3: any drift between
the submodule's contents and the committed
`events/`/`objects/`/`enums/` output fails CI with a one-line
fix message.

## Local-edit policy

No local edits to the submodule. If the upstream schema has
a defect that blocks codegen, the fix lives in either:

- The codegen tool (`internal/gen/`), if the defect is
  representable as a special-case in the reader/emitter.
- An upstream PR to `ocsf/ocsf-schema`, after which the
  submodule advances to a commit containing the fix.

Patching the submodule directly leaves the local commit SHA
diverging from any upstream tag — no longer reproducible.

## Why `internal/`

The schema content is a codegen input, not a consumer-visible
artifact. Consumers of `go-ocsf` use the generated types in
`events/<category>/`, `objects/`, and `enums/`, plus the
hand-written core in the root `ocsf` package. The submodule
sits under `internal/` so external code can't take a
load-bearing dependency on the upstream's on-disk layout
(which is the upstream project's to change, not ours).

Code that needs the schema at runtime — for example, a
future runtime-validation feature that consults the
dictionary — would embed the relevant pieces via `//go:embed`
in the package that needs them, rather than re-exposing the
submodule path.

## License

The upstream OCSF schema is Apache-2.0, with attribution
preserved in `internal/schema/upstream/LICENSE` and
`internal/schema/upstream/NOTICE`. The library's own
root-level `LICENSE` covers the hand-written Go code and the
codegen output; the upstream license covers the JSON schema
content the codegen consumes.
