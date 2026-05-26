// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

package ocsf

import "encoding/json"

// Unmapped is the OCSF open-extension carrier: an attribute that
// holds arbitrary JSON the schema doesn't claim to model. The
// OCSF specification defines `unmapped` on every event class as
// the field a publisher uses to attach event-source-specific
// metadata that the standard hasn't normalized. Generated event
// structs declare their `unmapped` field as
// encoding/json.RawMessage so the bytes survive a Parse->Marshal
// round-trip unchanged (key order, whitespace, etc.). This type
// alias gives consumer code a more descriptive name when working
// with that field — `var extra ocsf.Unmapped` reads better than
// `var extra json.RawMessage` at the call site.
//
// Treat Unmapped as a leaf type: assignment, length, and
// json.Marshal / json.Unmarshal work; field access does not.
// Decode the nested bytes with a second json.Unmarshal pass when
// the consumer knows the source-specific shape, or pass through
// verbatim when forwarding.
//
// The OCSF design considers `unmapped` the canonical
// extension-carrier; library consumers should NOT introduce
// their own ad-hoc fields outside `unmapped` even when the
// publisher controls both ends of the wire. Doing so removes the
// schema's only well-known integration point for non-standard
// data and surprises downstream consumers that rely on
// `unmapped` to capture every non-schema value.
type Unmapped = json.RawMessage

// JSONNull is the JSON literal `null` as a json.RawMessage, ready
// to assign into an Unmapped field (or any other
// json.RawMessage-valued field) when the caller wants the wire
// output to carry an explicit null rather than the field being
// omitted.
//
// The encoding/json default for a nil json.RawMessage is to
// emit `null` when the surrounding field lacks `omitempty`, and
// to omit the field entirely when it does. JSONNull is the
// explicit form for callers who want `null` regardless of
// omitempty.
var JSONNull = json.RawMessage(`null`)
