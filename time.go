// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

package ocsf

import "time"

// OCSF timestamps are wire-typed as int64 milliseconds since the
// Unix epoch (`timestamp_t` in the schema). Generated event and
// object structs keep the int64 — see the design decision around
// "wire-stable round-trip" in the project docs — and consumers
// that want a Go time.Time use the helpers below.
//
// Two design points worth knowing:
//
//   - Auto-conversion is intentionally NOT done at unmarshal.
//     The library keeps the int64 on the wire so a byte-stable
//     round-trip (Parse -> Marshal) produces JSON identical to
//     the input. Lifting to time.Time at decode would also lose
//     the distinction between a missing field (zero) and an
//     epoch-valued field (also zero in time.Time).
//   - Millisecond precision matches OCSF's `timestamp_t`
//     definition; higher-resolution timestamps round-trip with
//     loss across the conversion. Consumers needing sub-ms
//     precision should keep the int64 and not call these
//     helpers.

// TimeFromMillis returns the time.Time corresponding to the OCSF
// timestamp ms (milliseconds since the Unix epoch, UTC). The
// special value ms == 0 maps to the Unix epoch
// (1970-01-01T00:00:00Z) and is the conventional zero-value of
// time.Time in OCSF interchange; callers that need to
// distinguish "absent" from "epoch" should check the field for
// zero before calling.
//
// The returned time.Time is in UTC. Convert to a local zone
// with time.Time.Local or time.Time.In as appropriate.
func TimeFromMillis(ms int64) time.Time {
	return time.UnixMilli(ms).UTC()
}

// MillisFromTime returns the OCSF wire representation of t —
// milliseconds since the Unix epoch, UTC. Sub-millisecond
// precision is truncated (not rounded) toward the epoch, which
// matches time.Time.UnixMilli's contract.
//
// A zero time.Time{} produces the int64 representation of the
// Unix epoch's millisecond-difference from time.Time's own zero
// year (year 1), which is a large negative number — typically
// not what callers want. Use the JSON omitempty discipline on
// the surrounding struct field to omit unset timestamps from
// the wire output rather than emitting the time.Time zero
// value.
func MillisFromTime(t time.Time) int64 {
	return t.UnixMilli()
}
