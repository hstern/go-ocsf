// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

package ocsf_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	ocsf "github.com/hstern/go-ocsf"
	"github.com/hstern/go-ocsf/events/iam"
	"github.com/hstern/go-ocsf/objects"
)

// ExampleParse demonstrates the canonical consumer flow:
// take a JSON-encoded OCSF event off the wire, Parse it into
// the typed event class via the registry, and read fields off
// the resulting struct.
//
// Parse dispatches on class_uid using factories registered by
// each events/<category>/ package's init() — importing one of
// those packages side-effects the registration into the
// dispatch table. Consumers should blank-import the
// categories they expect to see on the wire; payloads with
// class_uid in an unimported category route to BaseEvent (the
// forward-compat fallback documented in ExampleBaseEvent).
func ExampleParse() {
	wire := []byte(`{"class_uid":3002,"activity_id":1,"severity_id":1,` +
		`"metadata":{"product":{"name":"my-idp"},"version":"1.3.0"},` +
		`"time":1618524549901,"type_uid":300201,` +
		`"user":{"name":"alice"},"service":{"name":"ldap"},` +
		`"cloud":{"provider":"aws"},"osint":[{}]}`)

	e, err := ocsf.Parse(wire)
	if err != nil {
		fmt.Println("parse:", err)
		return
	}
	if a, ok := e.(*iam.Authentication); ok {
		fmt.Printf("user=%s class=%s class_uid=%d\n",
			a.User.Name, a.OCSFClassName(), a.OCSFClassUID())
	}
	// Output: user=alice class=Authentication class_uid=3002
}

// ExampleParse_baseEvent shows the forward-compat fallback.
// When Parse sees a class_uid the library doesn't recognize,
// the event lands as a *BaseEvent that preserves the raw
// bytes for forwarding. Consumers wanting "unknown class_uid
// is an error" do the type assertion explicitly.
func ExampleParse_baseEvent() {
	wire := []byte(`{"class_uid":99999,"class_name":"FutureClass"}`)

	e, err := ocsf.Parse(wire)
	if err != nil {
		fmt.Println("parse:", err)
		return
	}
	if be, ok := e.(*ocsf.BaseEvent); ok {
		fmt.Printf("unknown class %d (%s); raw bytes preserved\n",
			be.OCSFClassUID(), be.OCSFClassName())
	}
	// Output: unknown class 99999 (FutureClass); raw bytes preserved
}

// ExampleValidate demonstrates the typical validation flow
// after Parse. Validate returns the first violation as a
// *ValidationError; errors.As extracts the structured fields
// (Rule, Field, ClassUID) for programmatic handling.
//
// The example uses an Authentication missing its required
// `user` field, so Validate fires on the first
// alphabetically-sorted required field that's still missing.
func ExampleValidate() {
	a := iam.Authentication{
		// Cloud, Metadata, Osint, Service set — User omitted
		// to trigger the required-field violation.
		Cloud:    &objects.Cloud{Provider: "aws"},
		Metadata: &objects.Metadata{Product: &objects.Product{Name: "p"}, Version: "1.3.0"},
		Osint:    []objects.Osint{{}},
		Service:  &objects.Service{Name: "ldap"},
	}
	err := a.Validate()
	var verr *ocsf.ValidationError
	if errors.As(err, &verr) {
		fmt.Printf("rule=%s field=%s\n", verr.Rule, verr.Field)
	}
	// Output: rule=required field=user
}

// ExampleTimeFromMillis shows the int64 → time.Time
// conversion for the wire-format timestamp_t.
// MillisFromTime is the inverse, with truncation toward the
// epoch on sub-millisecond precision.
func ExampleTimeFromMillis() {
	const wireValue int64 = 1618524549901
	t := ocsf.TimeFromMillis(wireValue)
	fmt.Println(t.Format("2006-01-02T15:04:05.000Z"))
	// Output: 2021-04-15T22:09:09.901Z
}

// Example_roundTrip demonstrates the typical agent loop:
// receive bytes, Parse, optionally inspect or modify, then
// Marshal back. For known classes, the marshal output may add
// zero-valued keys where the wire payload was sparse — see
// the README's Stability section for the trade-off and
// OCSF-29 for the planned tightening. For unknown classes
// (BaseEvent), MarshalJSON returns the input bytes verbatim.
func Example_roundTrip() {
	wire := []byte(`{"class_uid":99999,"data":"forward me verbatim"}`)

	e, err := ocsf.Parse(wire)
	if err != nil {
		fmt.Println("parse:", err)
		return
	}
	out, err := json.Marshal(e)
	if err != nil {
		fmt.Println("marshal:", err)
		return
	}
	fmt.Println("verbatim:", bytes.Equal(wire, out))
	// Output: verbatim: true
}

// ExampleCategory_String shows the typed Category enum used
// for log-friendly category-name lookup.
func ExampleCategory_String() {
	fmt.Println(ocsf.CategoryIAM.String())
	// Output: Identity & Access Management
}
