// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

package ocsf

import "fmt"

// ValidationError reports a single rule violation discovered by
// a generated [Validate] method on an event class (or, later,
// by hand-written cross-field validators). Validate returns
// the first violation it finds; consumers wanting an
// exhaustive list of failures call Validate after fixing each
// reported field, or use a future []ValidationError-returning
// helper if one is added.
//
// ValidationError satisfies the error interface, so the
// idiomatic call shape works:
//
//	if err := event.Validate(); err != nil {
//	    var verr *ocsf.ValidationError
//	    if errors.As(err, &verr) {
//	        // verr.Field, verr.Rule, etc. for structured handling
//	    }
//	    return err
//	}
type ValidationError struct {
	// ClassUID is the OCSF class_uid of the event being
	// validated. Zero for [BaseEvent] (which doesn't validate
	// — it's the forward-compat fallback) and for hand-written
	// helper-level violations that aren't event-class-scoped.
	ClassUID int

	// Field is the OCSF attribute path that violated the rule,
	// in dot notation (e.g. "metadata", "user.name",
	// "finding_info.uid"). Always the wire-format snake_case
	// identifier, not the Go field name, so consumers can map
	// the error back to the schema and to the raw JSON.
	Field string

	// Rule names the violated rule family. Known values used
	// by the v0.1.x library:
	//
	//   "required"     — attribute is marked requirement:
	//                    "required" in the schema and is absent
	//                    or empty on the wire.
	//   "enum"         — *_id attribute carries a value outside
	//                    the dictionary's enum range, or
	//                    disagrees with its paired sibling
	//                    string. (Lands in OCSF-17.)
	//   "constraint"   — a class-level constraint like
	//                    at_least_one or just_one was violated.
	//                    (Lands in OCSF-20.)
	//
	// External code matching on Rule should use these string
	// constants verbatim; they're stable across the v0.1.x
	// line.
	Rule string

	// Reason is a short human-readable explanation of why the
	// rule fired. Suitable for log lines and operator-facing
	// error messages; not stable enough to switch on
	// programmatically.
	Reason string
}

// Error returns a colon-separated rendering suitable for log
// output and the default fmt.Errorf wrapping path.
func (e *ValidationError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("ocsf: class_uid=%d: %s: %s: %s", e.ClassUID, e.Field, e.Rule, e.Reason)
}
