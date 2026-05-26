# go-ocsf

A Go library implementing the [Open Cybersecurity Schema
Framework][ocsf-home] (OCSF) — typed encoder / decoder for OCSF events,
with a codegen pipeline driven by the upstream OCSF JSON schema.

[ocsf-home]: https://schema.ocsf.io/

## Status

**Pre-release.** First tag will be `v0.1.0`. APIs may shift until then.

The library currently targets **OCSF schema version 1.3.0**. The
`ocsf.SchemaVersion` constant is the single source of truth for the
wire-format version the generated types track.

## Install

```
go get github.com/hstern/go-ocsf@latest
```

Requires Go 1.26 or later. Zero non-stdlib runtime dependencies.

## Roadmap

A quickstart, emitter and consumer examples, and the full public API
catalog land in the `v0.1.0` polish phase. Until then, the package
surface should be considered exploratory.

- Codegen pipeline (`internal/gen/`): emits per-event-class and
  per-object structs from the vendored OCSF schema.
- Generated event classes under `events/<category>/`, generated
  objects under `objects/`.
- Hand-written core in the root `ocsf` package: `Event` interface,
  `BaseEvent`, `Parse`, registry, validation.
- Conformance: byte-stability round-trip over upstream OCSF sample
  events.

## License

Apache-2.0. See [LICENSE](LICENSE).
