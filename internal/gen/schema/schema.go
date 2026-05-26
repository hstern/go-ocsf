// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

// Package schema is the in-memory model of the OCSF metaschema
// produced by reading a vendored schema directory tree.
//
// Consumers of this package (the codegen tool in internal/gen) load
// a [Schema] from disk with [Load], then walk Schema.Events and
// Schema.Objects to emit Go types. The model captures everything
// the upstream JSON expresses about an attribute's type, requirement,
// enum values, profile membership, and constraints — extends and
// $include have already been resolved at load time, so each class's
// Attributes slice is the full attribute set the class actually
// exposes on the wire.
//
// The OCSF metaschema is not standard JSON Schema. Its inheritance
// (extends) and inclusion ($include) mechanisms are bespoke; see the
// load.go file in this package for the resolution rules. Don't
// reach for a generic JSON-Schema library to consume the upstream
// JSON — it won't model OCSF's semantics correctly.
package schema

// Schema is the fully-resolved OCSF metaschema for a single
// upstream schema version.
//
// All maps are keyed by the OCSF name (snake_case identifiers). The
// in-memory model is deterministic: maps used for lookup, slices
// used for iteration in name-sorted order.
type Schema struct {
	// Version is the upstream schema version this tree describes
	// (e.g. "1.3.0"), read from version.json.
	Version string

	// Categories holds the named categories from categories.json.
	Categories map[string]Category

	// Dictionary holds the attribute and type registry from
	// dictionary.json.
	Dictionary Dictionary

	// Profiles holds the profile bundles loaded from profiles/*.json,
	// keyed by profile name (e.g. "cloud", "host").
	Profiles map[string]Profile

	// Includes holds the include bundles loaded from includes/**/*.json,
	// keyed by their path relative to the schema root
	// (e.g. "includes/classification.json"). Path keys mirror the
	// strings used in attribute `$include` directives.
	Includes map[string]Include

	// Objects holds the resolved object definitions loaded from
	// objects/*.json, keyed by object name.
	Objects map[string]ObjectClass

	// Events holds the resolved event-class definitions loaded from
	// events/**/*.json plus events/base_event.json, keyed by class
	// name. Category-root abstract classes (iam, network, finding,
	// ...) are present alongside concrete classes; concrete classes
	// have a non-zero UID, abstract classes have UID == 0.
	Events map[string]EventClass
}

// Category is a single entry under categories.json — a top-level
// grouping of event classes (system, iam, network, ...).
type Category struct {
	// Name is the snake_case identifier (e.g. "iam").
	Name string

	Caption     string
	Description string

	// UID is the category's unique integer identifier (1..N).
	// Concrete event classes within this category report
	// `category_uid: UID` on the wire and their `class_uid` is
	// computed as `UID * 1000 + EventClass.UID`.
	UID int
}

// Dictionary is the central attribute and type registry. Every
// attribute referenced by an event class or object resolves its
// wire-level shape (type, default enum, observable tag) from here.
type Dictionary struct {
	// Attributes is the global attribute registry keyed by attribute
	// name. Event and object definitions reference attributes by
	// name and override only the per-class metadata (requirement,
	// group, sometimes description); the dictionary entry provides
	// the rest.
	Attributes map[string]DictAttr

	// Types is the primitive-type registry keyed by type name
	// (e.g. "string_t", "port_t", "timestamp_t"). The map includes
	// the seven non-derived primitives (boolean_t, integer_t,
	// long_t, float_t, json_t, string_t, plus the dictionary's
	// fixed value sets) and the derived types that alias them
	// (e.g. email_t -> string_t).
	Types map[string]TypeDef
}

// DictAttr is one entry in Dictionary.Attributes — the canonical
// attribute definition. Event and object class attributes inherit
// these fields and may override Caption, Description, Group,
// Requirement, Sibling, and Enum (the dictionary's Enum is the
// default; classes may add class-specific values).
type DictAttr struct {
	Name        string
	Caption     string
	Description string

	// Type is the dictionary type for this attribute. It is either
	// a primitive name from Dictionary.Types (e.g. "string_t",
	// "integer_t") or an object name from Schema.Objects (e.g.
	// "user", "endpoint"). If IsArray is true, the wire shape is
	// `[]Type`.
	Type string

	// IsArray reports whether the attribute is repeated on the wire.
	IsArray bool

	// Sibling is the paired string attribute for an enum id
	// attribute, if any (e.g. action_id's sibling is "action"). The
	// sibling is set to the enum value's caption when the id maps
	// to a known value, and to a free-form string when the id is
	// "Other" (99).
	Sibling string

	// Enum is the default set of valid integer values for this
	// attribute. Keyed by the string form of the integer
	// (preserving upstream key shape). Empty if not an enum.
	Enum map[string]EnumValue

	// SuppressChecks lists upstream lint suppressions for this
	// attribute (e.g. "enum_convention", "sibling_convention").
	// Carried for fidelity; codegen ignores them.
	SuppressChecks []string

	// Observable is the OCSF observable type id for this attribute,
	// or nil if not an observable. Observable attributes are
	// candidates for the observables[] enrichment field.
	Observable *int

	// Deprecated when set marks this attribute as deprecated by
	// upstream.
	Deprecated *Deprecated
}

// TypeDef is one entry in Dictionary.Types — a primitive or
// primitive-derived type definition.
type TypeDef struct {
	Name string

	Caption     string
	Description string

	// Type is the underlying primitive that a derived type aliases
	// (e.g. email_t.Type == "string_t"). Empty for the seven base
	// primitives.
	Type string

	// TypeName is the upstream human-readable name (e.g. "String",
	// "Integer", "Long"). Codegen uses this for diagnostics; the
	// actual Go-type mapping is decided by the emitter.
	TypeName string

	// MaxLen, when set, is the maximum length (in bytes for
	// strings) the upstream schema allows.
	MaxLen *int

	// Regex, when set, is a regex the wire value must match.
	Regex string

	// Range, when set, constrains a numeric value to [Range[0], Range[1]].
	Range *[2]float64

	// Values, when set, is the closed set of legal values
	// (e.g. boolean_t has [false, true]).
	Values []any

	// Observable, when set, is the observable type id this primitive
	// participates in.
	Observable *int
}

// EnumValue is one entry in an attribute's enum map.
type EnumValue struct {
	// ID is the original upstream key (string form of the integer
	// id), preserved verbatim so codegen can emit the right
	// constant value.
	ID string

	Caption     string
	Description string
}

// Profile is one entry in Schema.Profiles — a reusable bundle of
// attribute metadata applied to event classes or objects that
// declare the profile in their Profiles list.
type Profile struct {
	Name        string
	Caption     string
	Description string

	// Annotations holds upstream-tagged hints (e.g. group =
	// "primary"); codegen does not consume these directly but they
	// travel with the profile for fidelity.
	Annotations map[string]string

	// Attributes is the ordered list of attributes the profile
	// contributes when applied. Attribute Names are dictionary
	// references; the per-class merge resolves the dictionary type.
	Attributes []ClassAttr
}

// Include is one entry in Schema.Includes — a reusable attribute
// bundle referenced by classes via the $include directive. Unlike
// Profile, Include is not gated by a class's Profiles list;
// $include always merges unconditionally.
type Include struct {
	// Path is the relative path used in $include directives
	// (e.g. "includes/classification.json").
	Path string

	Caption     string
	Description string

	Annotations map[string]string

	Attributes []ClassAttr
}

// ObjectClass is one entry in Schema.Objects — an OCSF object
// definition with extends and $include already resolved.
type ObjectClass struct {
	Name        string
	Caption     string
	Description string

	// Extends is the parent object name (e.g. "_entity", "object"),
	// empty for the root abstract "object" type. Resolved already:
	// Attributes is the merged set including all ancestors.
	Extends string

	// Observable, if set, is the OCSF observable type id for this
	// object kind (the object as a whole, not individual attributes).
	Observable *int

	// Profiles lists the profile names this object admits.
	Profiles []string

	// Constraints captures the at_least_one and just_one rules
	// upstream declared on this object.
	Constraints Constraints

	// Deprecated when set marks the object as deprecated.
	Deprecated *Deprecated

	// Attributes is the fully-resolved attribute list:
	// ancestor attributes, then $include bundles, then local
	// definitions, in upstream-merge order with later writes
	// overriding earlier ones. Sorted by Name for determinism.
	Attributes []ClassAttr
}

// EventClass is one entry in Schema.Events — an OCSF event class
// definition with extends and $include already resolved.
type EventClass struct {
	Name        string
	Caption     string
	Description string

	// Category is the category name this class belongs to (e.g.
	// "iam"). The special value "other" is reserved for the
	// abstract base_event.
	Category string

	// Extends is the parent class name (often a category-root
	// abstract class like "iam" or "network", which itself extends
	// "base_event"). Empty only for base_event.
	Extends string

	// UID is the class identifier within its category. The wire
	// class_uid is Category-UID * 1000 + UID (or 0 for abstract
	// classes such as base_event, iam, network where UID is 0).
	// Use ClassUID for the computed value.
	UID int

	Profiles []string

	Constraints Constraints

	// Associations is the upstream-declared semantic linkages
	// between fields (e.g. actor.user <-> src_endpoint).
	Associations map[string][]string

	// Observables is the per-class override map of attribute-path
	// to observable type id.
	Observables map[string]int

	Deprecated *Deprecated

	Attributes []ClassAttr
}

// ClassAttr is one attribute as it appears on a resolved event
// class or object. Combines fields from the dictionary entry
// (Type, IsArray, defaults) with the per-class overrides
// (Requirement, Group, sometimes Description).
type ClassAttr struct {
	Name        string
	Caption     string
	Description string

	// Group is one of "context", "classification", "occurrence",
	// "primary", or empty if upstream did not assign one.
	Group string

	// Requirement is one of "optional", "recommended", "required",
	// or empty if upstream did not assign one. Codegen treats empty
	// as "optional" for Validate generation purposes.
	Requirement string

	// Sibling is the paired string attribute name for an enum id
	// (e.g. action_id.Sibling == "action").
	Sibling string

	// Enum is the merged enum value set for this attribute on this
	// class — dictionary defaults plus any per-class additions.
	// Empty if not an enum attribute.
	Enum map[string]EnumValue

	// Observable, if set, is the observable type id contributed by
	// the dictionary or per-class override.
	Observable *int

	// Profile, if set, marks the attribute as gated by a specific
	// profile name. The value "" means unconditional; "null" in
	// upstream JSON erases the attribute from the resolved set and
	// such entries do not appear in Attributes.
	Profile string

	// Type is the dictionary type for this attribute, resolved at
	// load time (e.g. "string_t", "user").
	Type string

	// IsArray reports whether the attribute is repeated on the wire.
	IsArray bool

	Deprecated *Deprecated
}

// Constraints captures the upstream-declared cross-field rules on
// an object or event class.
type Constraints struct {
	// AtLeastOne lists attribute names of which at least one must
	// be present for the class instance to be valid.
	AtLeastOne []string

	// JustOne lists attribute names of which exactly one must be
	// present.
	JustOne []string
}

// Deprecated captures the upstream @deprecated marker. The exact
// shape is documented by upstream as `{message, since}` but kept
// as a struct for forward-compatibility.
type Deprecated struct {
	Message string
	Since   string
}

// ClassUID returns the wire-format class_uid for c, computed as
// CategoryUID*1000 + c.UID, looking up the category by name from
// s.Categories. Returns 0 for abstract classes (UID == 0) and for
// classes whose category is "other".
func (s *Schema) ClassUID(c EventClass) int {
	if c.UID == 0 {
		return 0
	}
	cat, ok := s.Categories[c.Category]
	if !ok {
		return 0
	}
	return cat.UID*1000 + c.UID
}
