// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// Load reads the vendored OCSF schema rooted at the given path on
// the host filesystem and returns the fully-resolved [Schema].
// All extends chains and $include directives are resolved before
// Load returns; consumers walk the result without further
// recursion.
//
// Load is a thin wrapper around [LoadFS] that opens the path as
// an [os.DirFS] rooted at root.
func Load(root string) (*Schema, error) {
	return LoadFS(rootFS{root: root})
}

// LoadFS reads a vendored OCSF schema from the given [fs.FS].
// The filesystem must contain at minimum version.json,
// dictionary.json, categories.json, and the objects/, events/,
// includes/, and profiles/ directories.
//
// Extension subtrees (extensions/*) are vendored upstream but
// intentionally skipped here: v0.1.0 ships only the base schema.
// A future PR can lift this skip when extension codegen lands.
func LoadFS(fsys fs.FS) (*Schema, error) {
	s := &Schema{
		Categories: map[string]Category{},
		Profiles:   map[string]Profile{},
		Includes:   map[string]Include{},
		Objects:    map[string]ObjectClass{},
		Events:     map[string]EventClass{},
		Dictionary: Dictionary{
			Attributes: map[string]DictAttr{},
			Types:      map[string]TypeDef{},
		},
	}

	if err := loadVersion(fsys, s); err != nil {
		return nil, fmt.Errorf("version.json: %w", err)
	}
	if err := loadCategories(fsys, s); err != nil {
		return nil, fmt.Errorf("categories.json: %w", err)
	}
	if err := loadDictionary(fsys, s); err != nil {
		return nil, fmt.Errorf("dictionary.json: %w", err)
	}
	if err := loadIncludes(fsys, s); err != nil {
		return nil, fmt.Errorf("includes: %w", err)
	}
	if err := loadProfiles(fsys, s); err != nil {
		return nil, fmt.Errorf("profiles: %w", err)
	}

	rawObjects, err := loadRawObjects(fsys)
	if err != nil {
		return nil, fmt.Errorf("objects: %w", err)
	}
	rawEvents, err := loadRawEvents(fsys)
	if err != nil {
		return nil, fmt.Errorf("events: %w", err)
	}

	r := &resolver{
		schema:     s,
		rawObjects: rawObjects,
		rawEvents:  rawEvents,
		objects:    map[string]ObjectClass{},
		events:     map[string]EventClass{},
		visiting:   map[string]bool{},
	}
	if err := r.resolveAll(); err != nil {
		return nil, err
	}
	s.Objects = r.objects
	s.Events = r.events

	return s, nil
}

// rootFS is a minimal fs.FS rooted at a host filesystem path.
// We deliberately avoid os.DirFS so the loader's only dependency
// on the host filesystem lives in one named type and tests can
// substitute an fstest.MapFS without ceremony.
type rootFS struct{ root string }

func (r rootFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	return openHostFile(filepath.Join(r.root, filepath.FromSlash(name)))
}

// versionFile is the on-disk shape of version.json.
type versionFile struct {
	Version string `json:"version"`
}

func loadVersion(fsys fs.FS, s *Schema) error {
	var v versionFile
	if err := readJSON(fsys, "version.json", &v); err != nil {
		return err
	}
	if v.Version == "" {
		return errors.New("missing version field")
	}
	s.Version = v.Version
	return nil
}

// categoriesFile is the on-disk shape of categories.json.
type categoriesFile struct {
	Attributes map[string]struct {
		Caption     string `json:"caption"`
		Description string `json:"description"`
		UID         int    `json:"uid"`
	} `json:"attributes"`
}

func loadCategories(fsys fs.FS, s *Schema) error {
	var c categoriesFile
	if err := readJSON(fsys, "categories.json", &c); err != nil {
		return err
	}
	for name, body := range c.Attributes {
		s.Categories[name] = Category{
			Name:        name,
			Caption:     body.Caption,
			Description: body.Description,
			UID:         body.UID,
		}
	}
	return nil
}

// dictionaryFile is the on-disk shape of dictionary.json. The
// `attributes` map mixes regular DictAttr entries (most keys) with
// nothing else; the `types` block carries the primitive registry.
type dictionaryFile struct {
	Attributes map[string]dictAttrRaw `json:"attributes"`
	Types      struct {
		Attributes map[string]typeDefRaw `json:"attributes"`
	} `json:"types"`
}

type dictAttrRaw struct {
	Caption        string             `json:"caption"`
	Description    string             `json:"description"`
	Type           string             `json:"type"`
	IsArray        bool               `json:"is_array"`
	Sibling        string             `json:"sibling"`
	Enum           map[string]enumRaw `json:"enum"`
	SuppressChecks []string           `json:"suppress_checks"`
	Observable     *int               `json:"observable"`
	Deprecated     *deprecatedRaw     `json:"@deprecated"`
}

type typeDefRaw struct {
	Caption     string         `json:"caption"`
	Description string         `json:"description"`
	Type        string         `json:"type"`
	TypeName    string         `json:"type_name"`
	MaxLen      *int           `json:"max_len"`
	Regex       string         `json:"regex"`
	Range       []float64      `json:"range"`
	Values      []any          `json:"values"`
	Observable  *int           `json:"observable"`
	Deprecated  *deprecatedRaw `json:"@deprecated"`
}

type enumRaw struct {
	Caption     string `json:"caption"`
	Description string `json:"description"`
}

type deprecatedRaw struct {
	Message string `json:"message"`
	Since   string `json:"since"`
}

func (d *deprecatedRaw) toModel() *Deprecated {
	if d == nil {
		return nil
	}
	return &Deprecated{Message: d.Message, Since: d.Since}
}

func enumsToModel(in map[string]enumRaw) map[string]EnumValue {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]EnumValue, len(in))
	for k, v := range in {
		out[k] = EnumValue{ID: k, Caption: v.Caption, Description: v.Description}
	}
	return out
}

func loadDictionary(fsys fs.FS, s *Schema) error {
	var d dictionaryFile
	if err := readJSON(fsys, "dictionary.json", &d); err != nil {
		return err
	}
	for name, raw := range d.Attributes {
		s.Dictionary.Attributes[name] = DictAttr{
			Name:           name,
			Caption:        raw.Caption,
			Description:    raw.Description,
			Type:           raw.Type,
			IsArray:        raw.IsArray,
			Sibling:        raw.Sibling,
			Enum:           enumsToModel(raw.Enum),
			SuppressChecks: append([]string(nil), raw.SuppressChecks...),
			Observable:     raw.Observable,
			Deprecated:     raw.Deprecated.toModel(),
		}
	}
	for name, raw := range d.Types.Attributes { //nolint:gocritic // copy acceptable in one-shot load
		t := TypeDef{
			Name:        name,
			Caption:     raw.Caption,
			Description: raw.Description,
			Type:        raw.Type,
			TypeName:    raw.TypeName,
			MaxLen:      raw.MaxLen,
			Regex:       raw.Regex,
			Values:      append([]any(nil), raw.Values...),
			Observable:  raw.Observable,
		}
		if len(raw.Range) == 2 {
			t.Range = &[2]float64{raw.Range[0], raw.Range[1]}
		}
		s.Dictionary.Types[name] = t
	}
	return nil
}

// includeFile is the on-disk shape of an includes/*.json or
// profiles/*.json file (the two share enough structure that the
// raw decode is unified). meta="profile" distinguishes profiles.
type includeFile struct {
	Caption     string                     `json:"caption"`
	Description string                     `json:"description"`
	Name        string                     `json:"name"`
	Meta        string                     `json:"meta"`
	Annotations map[string]string          `json:"annotations"`
	Attributes  map[string]json.RawMessage `json:"attributes"`
}

func loadIncludes(fsys fs.FS, s *Schema) error {
	return walkJSON(fsys, "includes", func(p string) error {
		var f includeFile
		if err := readJSON(fsys, p, &f); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		attrs, err := decodeClassAttrs(f.Attributes)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		s.Includes[p] = Include{
			Path:        p,
			Caption:     f.Caption,
			Description: f.Description,
			Annotations: f.Annotations,
			Attributes:  attrs,
		}
		return nil
	})
}

func loadProfiles(fsys fs.FS, s *Schema) error {
	return walkJSON(fsys, "profiles", func(p string) error {
		var f includeFile
		if err := readJSON(fsys, p, &f); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		attrs, err := decodeClassAttrs(f.Attributes)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		name := f.Name
		if name == "" {
			// Profiles without an explicit name use the file's
			// basename (without extension) by convention; preserve
			// upstream's implicit naming.
			name = strings.TrimSuffix(path.Base(p), ".json")
		}
		s.Profiles[name] = Profile{
			Name:        name,
			Caption:     f.Caption,
			Description: f.Description,
			Annotations: f.Annotations,
			Attributes:  attrs,
		}
		// The $include directive references profiles by their full
		// "profiles/<name>.json" path, so also index them in
		// Schema.Includes for the resolver's $include lookup. This
		// matches upstream behavior — profiles are mechanically a
		// kind of include.
		s.Includes[p] = Include{
			Path:        p,
			Caption:     f.Caption,
			Description: f.Description,
			Annotations: f.Annotations,
			Attributes:  attrs,
		}
		return nil
	})
}

// classBody is the on-disk shape shared by event and object class
// JSON files. Fields not used by all class kinds (e.g. UID, Category,
// Associations, Observables) are silently zero when absent.
type classBody struct {
	Caption      string                     `json:"caption"`
	Description  string                     `json:"description"`
	Name         string                     `json:"name"`
	Extends      string                     `json:"extends"`
	Category     string                     `json:"category"`
	UID          int                        `json:"uid"`
	Observable   *int                       `json:"observable"`
	Profiles     []string                   `json:"profiles"`
	Constraints  *constraintsRaw            `json:"constraints"`
	Attributes   map[string]json.RawMessage `json:"attributes"`
	Associations map[string][]string        `json:"associations"`
	Observables  map[string]int             `json:"observables"`
	Deprecated   *deprecatedRaw             `json:"@deprecated"`
}

type constraintsRaw struct {
	AtLeastOne []string `json:"at_least_one"`
	JustOne    []string `json:"just_one"`
}

func (c *constraintsRaw) toModel() Constraints {
	if c == nil {
		return Constraints{}
	}
	return Constraints{
		AtLeastOne: append([]string(nil), c.AtLeastOne...),
		JustOne:    append([]string(nil), c.JustOne...),
	}
}

// rawObject and rawEvent retain the parsed file body plus its
// path, used during the resolution pass.
type rawObject struct {
	path string
	body classBody
}

type rawEvent struct {
	path string
	body classBody
}

func loadRawObjects(fsys fs.FS) (map[string]rawObject, error) {
	out := map[string]rawObject{}
	err := walkJSON(fsys, "objects", func(p string) error {
		var b classBody
		if err := readJSON(fsys, p, &b); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		name := b.Name
		if name == "" {
			name = strings.TrimSuffix(path.Base(p), ".json")
		}
		if existing, ok := out[name]; ok {
			return fmt.Errorf("duplicate object %q at %s and %s", name, existing.path, p)
		}
		out[name] = rawObject{path: p, body: b}
		return nil
	})
	return out, err
}

func loadRawEvents(fsys fs.FS) (map[string]rawEvent, error) {
	out := map[string]rawEvent{}
	err := walkJSON(fsys, "events", func(p string) error {
		var b classBody
		if err := readJSON(fsys, p, &b); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		name := b.Name
		if name == "" {
			name = strings.TrimSuffix(path.Base(p), ".json")
		}
		if existing, ok := out[name]; ok {
			return fmt.Errorf("duplicate event %q at %s and %s", name, existing.path, p)
		}
		out[name] = rawEvent{path: p, body: b}
		return nil
	})
	return out, err
}

// resolver carries the state for the extends + $include resolution
// pass. Cycles in extends are detected via the visiting set.
type resolver struct {
	schema     *Schema
	rawObjects map[string]rawObject
	rawEvents  map[string]rawEvent

	objects map[string]ObjectClass
	events  map[string]EventClass

	visiting map[string]bool
}

func (r *resolver) resolveAll() error {
	// Object names sorted for determinism.
	objNames := make([]string, 0, len(r.rawObjects))
	for n := range r.rawObjects {
		objNames = append(objNames, n)
	}
	sort.Strings(objNames)
	for _, n := range objNames {
		if _, err := r.resolveObject(n); err != nil {
			return err
		}
	}

	evtNames := make([]string, 0, len(r.rawEvents))
	for n := range r.rawEvents {
		evtNames = append(evtNames, n)
	}
	sort.Strings(evtNames)
	for _, n := range evtNames {
		if _, err := r.resolveEvent(n); err != nil {
			return err
		}
	}
	return nil
}

// resolveObject builds the resolved ObjectClass for name, memoizing
// the result. The root abstract "object" type has no on-disk JSON
// in some schema versions and is synthesized as an empty class
// when referenced as a parent.
func (r *resolver) resolveObject(name string) (ObjectClass, error) {
	if oc, ok := r.objects[name]; ok {
		return oc, nil
	}
	if r.visiting["object:"+name] {
		return ObjectClass{}, fmt.Errorf("extends cycle through object %q", name)
	}
	raw, ok := r.rawObjects[name]
	if !ok {
		// Synthesize the abstract root if referenced.
		if name == "object" {
			oc := ObjectClass{Name: "object"}
			r.objects[name] = oc
			return oc, nil
		}
		return ObjectClass{}, fmt.Errorf("unknown object %q", name)
	}
	r.visiting["object:"+name] = true
	defer delete(r.visiting, "object:"+name)

	// Start from parent's attributes, if any.
	attrs := map[string]ClassAttr{}
	if raw.body.Extends != "" {
		parent, err := r.resolveObject(raw.body.Extends)
		if err != nil {
			return ObjectClass{}, fmt.Errorf("object %q: %w", name, err)
		}
		for _, a := range parent.Attributes { //nolint:gocritic // copy acceptable in one-shot load
			attrs[a.Name] = a
		}
	}

	// Apply $include and local attribute overrides from this class.
	if err := r.mergeAttributes(raw.body.Attributes, attrs); err != nil {
		return ObjectClass{}, fmt.Errorf("object %q: %w", name, err)
	}

	oc := ObjectClass{
		Name:        name,
		Caption:     raw.body.Caption,
		Description: raw.body.Description,
		Extends:     raw.body.Extends,
		Observable:  raw.body.Observable,
		Profiles:    append([]string(nil), raw.body.Profiles...),
		Constraints: raw.body.Constraints.toModel(),
		Deprecated:  raw.body.Deprecated.toModel(),
		Attributes:  sortedAttrs(attrs),
	}
	r.objects[name] = oc
	return oc, nil
}

func (r *resolver) resolveEvent(name string) (EventClass, error) {
	if ec, ok := r.events[name]; ok {
		return ec, nil
	}
	if r.visiting["event:"+name] {
		return EventClass{}, fmt.Errorf("extends cycle through event %q", name)
	}
	raw, ok := r.rawEvents[name]
	if !ok {
		return EventClass{}, fmt.Errorf("unknown event %q", name)
	}
	r.visiting["event:"+name] = true
	defer delete(r.visiting, "event:"+name)

	attrs := map[string]ClassAttr{}
	// category and profiles are inherited from the parent class
	// unless the child JSON sets them explicitly; everything else
	// (caption, description, uid, associations, observables,
	// constraints) stays local. This mirrors upstream behavior:
	// authentication.json has no category field but is in the iam
	// category because it extends iam.
	inheritedCategory := ""
	var inheritedProfiles []string
	if raw.body.Extends != "" {
		parent, err := r.resolveEvent(raw.body.Extends)
		if err != nil {
			return EventClass{}, fmt.Errorf("event %q: %w", name, err)
		}
		for _, a := range parent.Attributes { //nolint:gocritic // copy acceptable in one-shot load
			attrs[a.Name] = a
		}
		inheritedCategory = parent.Category
		inheritedProfiles = parent.Profiles
	}

	if err := r.mergeAttributes(raw.body.Attributes, attrs); err != nil {
		return EventClass{}, fmt.Errorf("event %q: %w", name, err)
	}

	category := raw.body.Category
	if category == "" {
		category = inheritedCategory
	}
	profiles := append([]string(nil), raw.body.Profiles...)
	if profiles == nil {
		profiles = append([]string(nil), inheritedProfiles...)
	}

	ec := EventClass{
		Name:         name,
		Caption:      raw.body.Caption,
		Description:  raw.body.Description,
		Category:     category,
		Extends:      raw.body.Extends,
		UID:          raw.body.UID,
		Profiles:     profiles,
		Constraints:  raw.body.Constraints.toModel(),
		Associations: copyStringSliceMap(raw.body.Associations),
		Observables:  copyIntMap(raw.body.Observables),
		Deprecated:   raw.body.Deprecated.toModel(),
		Attributes:   sortedAttrs(attrs),
	}
	r.events[name] = ec
	return ec, nil
}

// mergeAttributes applies a class body's `attributes` block to the
// accumulating set. The $include directive (if any) merges in
// referenced include/profile bundles first; then each local
// attribute body is merged. Per-class merges override existing
// fields without erasing untouched ones; `profile: null` removes
// the attribute from the set entirely.
func (r *resolver) mergeAttributes(raw map[string]json.RawMessage, attrs map[string]ClassAttr) error {
	if raw == nil {
		return nil
	}
	// $include is processed first so local overrides take precedence
	// over included material.
	if rawInc, ok := raw["$include"]; ok {
		var paths []string
		if err := json.Unmarshal(rawInc, &paths); err != nil {
			return fmt.Errorf("$include: %w", err)
		}
		for _, p := range paths {
			inc, ok := r.schema.Includes[p]
			if !ok {
				return fmt.Errorf("$include: unknown path %q", p)
			}
			for _, a := range inc.Attributes { //nolint:gocritic // copy acceptable in one-shot load
				mergeAttr(attrs, a)
			}
		}
	}

	// Deterministic order of local keys for predictable diagnostics.
	keys := make([]string, 0, len(raw))
	for k := range raw {
		if k == "$include" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		var body classAttrRaw
		// Sentinel for explicit null in the JSON (some upstream
		// classes set `"action": null` to disable an inherited
		// attribute). json.Unmarshal of "null" into a non-pointer
		// struct silently no-ops, so check the raw bytes.
		if isJSONNull(raw[k]) {
			delete(attrs, k)
			continue
		}
		if err := json.Unmarshal(raw[k], &body); err != nil {
			return fmt.Errorf("attribute %q: %w", k, err)
		}
		// `profile: null` is the OCSF convention for removing an
		// inherited attribute that came in via a profile include.
		// json.Unmarshal won't distinguish "missing" from "null"
		// without a pointer, so we sniff the raw bytes.
		if hasJSONFieldNull(raw[k], "profile") {
			delete(attrs, k)
			continue
		}
		a := body.toClassAttr(k)
		mergeAttr(attrs, a)
	}

	// Resolve dictionary types for everything in the merged set.
	for k, a := range attrs { //nolint:gocritic // copy acceptable in one-shot load

		if a.Type != "" && a.IsArray {
			// Local override carries both — keep as-is.
			continue
		}
		da, ok := r.schema.Dictionary.Attributes[k]
		if !ok {
			// Unknown attribute names happen for include bundles
			// that introduce names not in dictionary.json. Leave
			// the resolved type empty; codegen will surface the
			// gap as a lint failure rather than silently emitting
			// a typeless field.
			attrs[k] = a
			continue
		}
		if a.Type == "" {
			a.Type = da.Type
			a.IsArray = da.IsArray
		}
		if a.Sibling == "" {
			a.Sibling = da.Sibling
		}
		if a.Caption == "" {
			a.Caption = da.Caption
		}
		if a.Description == "" {
			a.Description = da.Description
		}
		if a.Observable == nil {
			a.Observable = da.Observable
		}
		// Enum merge: dictionary defaults plus class additions.
		if len(da.Enum) > 0 {
			merged := make(map[string]EnumValue, len(da.Enum)+len(a.Enum))
			for k2, v := range da.Enum {
				merged[k2] = v
			}
			for k2, v := range a.Enum {
				merged[k2] = v
			}
			a.Enum = merged
		}
		attrs[k] = a
	}

	return nil
}

// classAttrRaw is the on-disk shape of a single attribute body
// within a class's `attributes` block.
type classAttrRaw struct {
	Caption     string             `json:"caption"`
	Description string             `json:"description"`
	Group       string             `json:"group"`
	Requirement string             `json:"requirement"`
	Sibling     string             `json:"sibling"`
	Enum        map[string]enumRaw `json:"enum"`
	Observable  *int               `json:"observable"`
	Profile     *string            `json:"profile"`
	Type        string             `json:"type"`
	IsArray     bool               `json:"is_array"`
	Deprecated  *deprecatedRaw     `json:"@deprecated"`
}

// decodeClassAttrs decodes the `attributes` block of an include or
// profile file into an ordered (name-sorted) slice of ClassAttr.
// The $include key is rejected — includes themselves don't nest
// $include; only events, objects, and profiles do.
func decodeClassAttrs(raw map[string]json.RawMessage) ([]ClassAttr, error) {
	if raw == nil {
		return nil, nil
	}
	keys := make([]string, 0, len(raw))
	for k := range raw {
		if k == "$include" {
			// Includes don't recursively include other includes in
			// the 1.3.0 schema. If a future schema version starts
			// using this, lift the restriction in the resolver too.
			return nil, fmt.Errorf("nested $include is not supported in include / profile bodies")
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]ClassAttr, 0, len(keys))
	for _, k := range keys {
		var body classAttrRaw
		if isJSONNull(raw[k]) {
			// Null at the top-level of an include is meaningless;
			// skip.
			continue
		}
		if err := json.Unmarshal(raw[k], &body); err != nil {
			return nil, fmt.Errorf("attribute %q: %w", k, err)
		}
		out = append(out, body.toClassAttr(k))
	}
	return out, nil
}

func (b classAttrRaw) toClassAttr(name string) ClassAttr {
	a := ClassAttr{
		Name:        name,
		Caption:     b.Caption,
		Description: b.Description,
		Group:       b.Group,
		Requirement: b.Requirement,
		Sibling:     b.Sibling,
		Enum:        enumsToModel(b.Enum),
		Observable:  b.Observable,
		Type:        b.Type,
		IsArray:     b.IsArray,
		Deprecated:  b.Deprecated.toModel(),
	}
	if b.Profile != nil {
		a.Profile = *b.Profile
	}
	return a
}

// mergeAttr writes a into attrs[a.Name], overwriting only the
// fields a explicitly sets (non-zero strings, non-nil pointers,
// non-empty maps). Untouched fields preserve the previous value.
func mergeAttr(attrs map[string]ClassAttr, a ClassAttr) {
	cur, ok := attrs[a.Name]
	if !ok {
		attrs[a.Name] = a
		return
	}
	if a.Caption != "" {
		cur.Caption = a.Caption
	}
	if a.Description != "" {
		cur.Description = a.Description
	}
	if a.Group != "" {
		cur.Group = a.Group
	}
	if a.Requirement != "" {
		cur.Requirement = a.Requirement
	}
	if a.Sibling != "" {
		cur.Sibling = a.Sibling
	}
	if a.Type != "" {
		cur.Type = a.Type
	}
	if a.IsArray {
		cur.IsArray = a.IsArray
	}
	if a.Observable != nil {
		cur.Observable = a.Observable
	}
	if a.Profile != "" {
		cur.Profile = a.Profile
	}
	if a.Deprecated != nil {
		cur.Deprecated = a.Deprecated
	}
	if len(a.Enum) > 0 {
		if cur.Enum == nil {
			cur.Enum = map[string]EnumValue{}
		}
		for k, v := range a.Enum {
			cur.Enum[k] = v
		}
	}
	attrs[a.Name] = cur
}

func sortedAttrs(attrs map[string]ClassAttr) []ClassAttr {
	out := make([]ClassAttr, 0, len(attrs))
	for _, a := range attrs { //nolint:gocritic // copy acceptable in one-shot load
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func copyStringSliceMap(m map[string][]string) map[string][]string {
	if m == nil {
		return nil
	}
	out := make(map[string][]string, len(m))
	for k, v := range m {
		out[k] = append([]string(nil), v...)
	}
	return out
}

func copyIntMap(m map[string]int) map[string]int {
	if m == nil {
		return nil
	}
	out := make(map[string]int, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// isJSONNull reports whether raw is the JSON literal `null` (after
// trimming insignificant whitespace).
func isJSONNull(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	return s == "null"
}

// hasJSONFieldNull reports whether raw is a JSON object containing
// field set to the literal `null`. Used to detect upstream's
// `profile: null` removal sentinel without a struct re-decode.
func hasJSONFieldNull(raw json.RawMessage, field string) bool {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return false
	}
	v, ok := m[field]
	if !ok {
		return false
	}
	return isJSONNull(v)
}
