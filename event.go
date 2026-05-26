// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

package ocsf

import "encoding/json"

// Event is the polymorphic interface implemented by every OCSF
// event class — both the generated leaves (the Authentication
// struct in events/iam, the DetectionFinding struct in
// events/findings, ...) and the forward-compat [BaseEvent]
// fallback for class_uid values the library doesn't recognize
// natively.
//
// The method set carries the four classification accessors the
// codegen pipeline already emits on every generated event class.
// They're prefixed with OCSF to disambiguate from same-named
// struct fields that exist on those classes (every event class
// carries class_uid / class_name / category_uid / category_name
// as wire-format struct fields, inherited from base_event's
// classification include; Go doesn't permit a method and a
// field to share a name).
//
// Validate() is deliberately not part of Event in this phase.
// It lands as a separate addition in Phase 4 (validation) so
// the interface stays stable across the early-phase work, and
// callers that want validation can do a type assertion to a
// validating sub-interface or call package-level Validate()
// once that lands.
//
// The interface is OPEN by design: there's no sealed() method
// or unexported tag. External packages may implement Event for
// their own event types — useful for testing, mock event
// streams, or library extensions that introduce event shapes
// outside the canonical OCSF schema.
type Event interface {
	// OCSFClassUID returns the OCSF wire-format class_uid for
	// this event. For known classes it's the codegen-baked
	// constant (e.g. Authentication.OCSFClassUID() == 3002);
	// for [BaseEvent] it's whatever value arrived on the wire,
	// or 0 if class_uid was absent.
	OCSFClassUID() int

	// OCSFClassName returns the OCSF class_name (the
	// human-readable caption, e.g. "Authentication"). For
	// [BaseEvent] it's the wire value, or empty if absent.
	OCSFClassName() string

	// OCSFCategoryUID returns the OCSF category_uid (the
	// integer identifier for the event's category, e.g. 3 for
	// IAM). For [BaseEvent] it's the wire value, or 0 if
	// absent.
	OCSFCategoryUID() int

	// OCSFCategoryName returns the OCSF category_name (the
	// snake_case identifier, e.g. "iam"). For [BaseEvent] it's
	// the wire value, or empty if absent.
	OCSFCategoryName() string
}

// BaseEvent is the forward-compat fallback that captures OCSF
// events whose class_uid the library doesn't have a generated
// type for. The library uses it when [Parse] (Phase 3) sees an
// unfamiliar class_uid, so consumers reading event streams
// don't lose data when a publisher emits classes from a newer
// schema version than the library was built against.
//
// BaseEvent preserves the original JSON bytes verbatim — key
// order, whitespace, numeric precision — so a Parse → Marshal
// round-trip through a BaseEvent produces output
// byte-identical to the input. The classification metadata
// (class_uid, class_name, category_uid, category_name) is
// extracted into typed fields during UnmarshalJSON for fast
// access via the Event interface methods, but the source of
// truth for marshaling stays the raw byte buffer.
//
// Modification of a BaseEvent's wire content isn't supported
// in v0.1.x; consumers wanting to forward an event with
// changes should round-trip the raw bytes through
// encoding/json into a typed shape they control. A future
// release may add structured-modification helpers if a
// concrete need surfaces.
type BaseEvent struct {
	raw []byte

	classUID     int
	className    string
	categoryUID  int
	categoryName string
}

// Compile-time check that BaseEvent satisfies Event. If a
// future change adds a method to Event without also adding it
// to BaseEvent, this declaration breaks the build.
var _ Event = (*BaseEvent)(nil)

// OCSFClassUID returns the class_uid value that appeared on the
// wire for this event, or 0 if the field was absent.
func (b *BaseEvent) OCSFClassUID() int { return b.classUID }

// OCSFClassName returns the class_name value that appeared on
// the wire for this event, or "" if the field was absent.
func (b *BaseEvent) OCSFClassName() string { return b.className }

// OCSFCategoryUID returns the category_uid value that appeared
// on the wire for this event, or 0 if the field was absent.
func (b *BaseEvent) OCSFCategoryUID() int { return b.categoryUID }

// OCSFCategoryName returns the category_name value that
// appeared on the wire for this event, or "" if the field was
// absent.
func (b *BaseEvent) OCSFCategoryName() string { return b.categoryName }

// UnmarshalJSON captures the input bytes verbatim for later
// round-trip and extracts the four classification fields into
// the typed accessors. Field absence is not an error — a
// payload missing class_uid produces a BaseEvent whose
// OCSFClassUID() returns 0, leaving it to the consumer to
// decide whether that's tolerable.
func (b *BaseEvent) UnmarshalJSON(data []byte) error {
	b.raw = make([]byte, len(data))
	copy(b.raw, data)

	var info struct {
		ClassUID     *int    `json:"class_uid"`
		ClassName    *string `json:"class_name"`
		CategoryUID  *int    `json:"category_uid"`
		CategoryName *string `json:"category_name"`
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return err
	}
	if info.ClassUID != nil {
		b.classUID = *info.ClassUID
	}
	if info.ClassName != nil {
		b.className = *info.ClassName
	}
	if info.CategoryUID != nil {
		b.categoryUID = *info.CategoryUID
	}
	if info.CategoryName != nil {
		b.categoryName = *info.CategoryName
	}
	return nil
}

// MarshalJSON returns the raw bytes captured by UnmarshalJSON
// verbatim. A zero-value BaseEvent marshals to the empty JSON
// object `{}` rather than `null`, which is the more useful
// shape for downstream consumers expecting an event payload.
func (b *BaseEvent) MarshalJSON() ([]byte, error) {
	if len(b.raw) == 0 {
		return []byte("{}"), nil
	}
	out := make([]byte, len(b.raw))
	copy(out, b.raw)
	return out, nil
}

// Raw returns a copy of the JSON bytes captured by
// UnmarshalJSON. Useful for forwarding the event verbatim to
// another consumer without going through MarshalJSON's
// allocation, or for re-parsing the payload into a different
// typed shape than [Event].
//
// The returned slice is a defensive copy — callers can modify
// it freely without affecting the BaseEvent. Mutate the
// underlying event only by replacing the BaseEvent rather than
// editing through Raw.
func (b *BaseEvent) Raw() []byte {
	out := make([]byte, len(b.raw))
	copy(out, b.raw)
	return out
}
