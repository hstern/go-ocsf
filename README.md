# go-ocsf

A Go library implementing the [Open Cybersecurity Schema
Framework][ocsf-home] (OCSF) — typed encoder / decoder for OCSF
events, with a codegen pipeline driven by the upstream OCSF JSON
schema.

[ocsf-home]: https://schema.ocsf.io/

## Status

**Pre-release.** First tag will be `v0.1.0`. APIs may shift until
then. The library currently targets **OCSF schema version 1.8.0**
(see [Stability](#stability) below for the version-pin design).

## Install

```
go get github.com/hstern/go-ocsf@latest
```

Requires Go 1.26 or later. Zero non-stdlib runtime dependencies.

## Quickstart — consumer (parse + validate)

A consumer reads bytes from somewhere (a log file, a Kafka
topic, an HTTP body) and wants a typed Go event back. `Parse`
dispatches on `class_uid` to the matching codegen-emitted type;
unknown classes round-trip through `BaseEvent` for
forward-compat.

```go
package main

import (
    "fmt"
    "log"

    ocsf "github.com/hstern/go-ocsf"

    // Importing an events/<category>/ package side-effects the
    // codegen-emitted init() registrations into ocsf.Parse's
    // class dispatch table. Import the categories you care
    // about; payloads with class_uid in an unimported category
    // route to BaseEvent (forward-compat).
    "github.com/hstern/go-ocsf/events/iam"
)

func main() {
    wire := []byte(`{"class_uid":3002,"activity_id":1,"severity_id":1,
        "metadata":{"product":{"name":"my-idp"},"version":"1.8.0"},
        "time":1618524549901,"type_uid":300201,
        "user":{"name":"alice"},"service":{"name":"ldap"},
        "cloud":{"provider":"aws"},"osint":[{}]}`)

    e, err := ocsf.Parse(wire)
    if err != nil {
        log.Fatal(err)
    }
    if err := e.Validate(); err != nil {
        log.Fatalf("invalid OCSF event: %v", err)
    }
    if a, ok := e.(*iam.Authentication); ok {
        fmt.Printf("user %q authenticated via %s at %v\n",
            a.User.Name, a.Service.Name, ocsf.TimeFromMillis(a.Time))
    }
}
```

## Quickstart — emitter (construct + marshal)

An emitter has native data (an audit-log row, a SIEM
detection) and wants to publish it as an OCSF event. Construct
the codegen-emitted struct directly and `json.Marshal`.

```go
package main

import (
    "encoding/json"
    "fmt"
    "time"

    ocsf "github.com/hstern/go-ocsf"
    "github.com/hstern/go-ocsf/events/iam"
    "github.com/hstern/go-ocsf/objects"
)

func main() {
    event := iam.Authentication{
        ActivityID: 1, // Logon
        SeverityID: 1, // Informational
        Time:       ocsf.MillisFromTime(time.Now()),
        TypeUID:    300201,
        User:       &objects.User{Name: "alice"},
        Service:    &objects.Service{Name: "ldap"},
        Cloud:      &objects.Cloud{Provider: "aws"},
        Metadata: &objects.Metadata{
            Product: &objects.Product{Name: "my-idp"},
            Version: ocsf.SchemaVersion,
        },
        Osint: []objects.Osint{{}},
    }

    if err := event.Validate(); err != nil {
        panic(err)
    }
    wire, _ := json.Marshal(event)
    fmt.Println(string(wire))
}
```

`event.OCSFClassUID()`, `event.OCSFClassName()`, etc. return the
codegen-baked classification constants (Authentication →
`3002`, `"Authentication"`, `3`, `"iam"`). They prefix with
`OCSF` because every event class also carries `class_uid` /
`class_name` / `category_uid` / `category_name` as wire-format
struct fields inherited from the base_event classification
include, and Go forbids a method and a field on the same type
to share a name.

## Forward-compat — unknown class_uid

`Parse` routes an event whose `class_uid` isn't registered to a
`*BaseEvent` that preserves the raw bytes verbatim through
`MarshalJSON`. Forwarding agents stay transparent across schema
versions:

```go
e, _ := ocsf.Parse(unknownClassPayload)
if be, ok := e.(*ocsf.BaseEvent); ok {
    log.Printf("unknown class_uid=%d, forwarding raw", be.OCSFClassUID())
    forwardToDownstream(be) // MarshalJSON returns the original bytes
}
```

Strict callers that prefer "unknown class_uid is an error" do
the type assertion and reject; the library's lenient-decode
stance leaves the policy choice to the caller.

## Validation

Generated `Validate()` on each event class checks four rule
families derived from the schema:

- **required**: schema-declared required fields must be present
  (pointer types: non-nil; slices/strings/RawMessage: non-empty).
- **enum**: scalar `*_id` fields must lie in the union of
  dictionary defaults and per-class additions.
- **enum (sibling correspondence)**: when both `<x>_id` and
  `<x>` (the string sibling) are set on the wire, the string
  must match the upstream caption of the id value. The OCSF
  "Other" id (typically 99) is the documented escape valve and
  skips correspondence.
- **constraint**: class-level `at_least_one` and `just_one`
  constraints from the schema.

`Validate` returns the first violation as `*ocsf.ValidationError`,
which has `ClassUID`, `Field`, `Rule`, `Reason` fields suitable
for structured handling via `errors.As`.

## Stability

The library version (SemVer) and the OCSF schema version are
**independent**. The current schema-version pin is
`ocsf.SchemaVersion = "1.8.0"`.

- **Schema-version bumps are library-minor releases.** When
  upstream ships a new OCSF schema, a single PR re-vendors the
  schema, regenerates the events/objects/enums packages, and
  bumps the constant. Consumers needing a specific schema
  version pin a specific library version.

- **The `codegen-diff` CI gate protects against silent drift**
  between the committed generated files and what the vendored
  schema produces. If the generator changes or the schema is
  re-vendored without re-running codegen, CI fails.

- **`v0.x` line is API-unstable.** Breaking changes can land in
  any minor pre-`v1.0.0`. The first stable release will
  document specific API guarantees.

## Layout

```
ocsf/                          ← root: Event interface, Parse,
                                 BaseEvent, ValidationError,
                                 Category, time helpers
events/<category>/             ← generated per-class structs
                                 (iam.Authentication, ...)
objects/                       ← generated per-object structs
                                 (User, Metadata, ...)
enums/                         ← generated typed-int enums
                                 (Severity, Activity, ...)
internal/gen/                  ← codegen tool
internal/schema/upstream/      ← OCSF schema (git submodule
                                 pinned to ocsf/ocsf-schema)
internal/specfixtures/         ← sample events for conformance
internal/conformance/          ← round-trip / forward-compat /
                                 unmapped passthrough tests
```

## Contributing

See [AGENTS.md](AGENTS.md) for the contributor conventions
(file-header format, codegen rules, commit message style, CI
gates).

## License

Apache-2.0. See [LICENSE](LICENSE). The vendored OCSF schema
under `internal/schema/v<version>/` is also Apache-2.0, with
upstream attribution preserved in each vendored directory's
`LICENSE` and `NOTICE`.
