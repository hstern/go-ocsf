// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

package ocsf_test

import (
	"errors"
	"testing"

	ocsf "github.com/hstern/go-ocsf"
	"github.com/hstern/go-ocsf/events/findings"
	"github.com/hstern/go-ocsf/events/iam"
	"github.com/hstern/go-ocsf/objects"
)

// TestValidate_TableAcrossClasses exercises the composed
// required-field / enum / sibling / constraint rules across
// representative event classes from four different
// categories. Each row pairs a small mutating step against an
// otherwise-valid event with the violation it should produce.
//
// The base events are constructed by per-category helpers that
// satisfy every gate (required fields + enum-zero + sibling
// blank + constraint membership) so a row that mutates ONE
// field provokes ONE specific violation. The table form keeps
// the test additive — adding coverage for a new class is one
// helper + however many table rows the class warrants.
func TestValidate_TableAcrossClasses(t *testing.T) {
	type row struct {
		name      string
		event     ocsf.Event
		wantRule  string
		wantField string
		wantNil   bool
	}

	// Build the four baseline events.
	auth := func() iam.Authentication {
		return iam.Authentication{
			Cloud:    &objects.Cloud{Provider: "aws"},
			Metadata: &objects.Metadata{Product: &objects.Product{Name: "p"}, Version: "1.3.0"},
			Osint:    []objects.Osint{{}},
			User:     &objects.User{Name: "alice"},
			Service:  &objects.Service{Name: "ldap"},
		}
	}
	det := func() findings.DetectionFinding {
		return findings.DetectionFinding{
			Cloud:       &objects.Cloud{Provider: "aws"},
			Metadata:    &objects.Metadata{Product: &objects.Product{Name: "p"}, Version: "1.3.0"},
			Osint:       []objects.Osint{{}},
			FindingInfo: &objects.FindingInfo{UID: "f-123"},
		}
	}
	// Mutate-and-check helpers — apply f to a fresh baseline,
	// then return the mutated value as an Event.
	mutateAuth := func(f func(*iam.Authentication)) ocsf.Event { a := auth(); f(&a); return a }
	mutateDet := func(f func(*findings.DetectionFinding)) ocsf.Event { e := det(); f(&e); return e }

	rows := []row{
		// Baseline: each fully-populated event validates.
		{name: "auth/baseline valid", event: auth(), wantNil: true},
		{name: "detection/baseline valid", event: det(), wantNil: true},

		// Required-field violations.
		{
			name:      "auth missing user",
			event:     mutateAuth(func(a *iam.Authentication) { a.User = nil }),
			wantRule:  "required",
			wantField: "user",
		},
		{
			name:      "detection missing finding_info",
			event:     mutateDet(func(e *findings.DetectionFinding) { e.FindingInfo = nil }),
			wantRule:  "required",
			wantField: "finding_info",
		},

		// Enum-membership violations.
		{
			name:      "auth activity_id out of range",
			event:     mutateAuth(func(a *iam.Authentication) { a.ActivityID = 50 }),
			wantRule:  "enum",
			wantField: "activity_id",
		},

		// Sibling-correspondence violations.
		{
			name: "auth activity_name mismatch",
			event: mutateAuth(func(a *iam.Authentication) {
				a.ActivityID = 1
				a.ActivityName = "Bogus"
			}),
			wantRule:  "enum",
			wantField: "activity_name",
		},

		// "Other" escape skips correspondence.
		{
			name: "auth activity_id=99 free-form sibling",
			event: mutateAuth(func(a *iam.Authentication) {
				a.ActivityID = 99
				a.ActivityName = "vendor-specific verb"
			}),
			wantNil: true,
		},

		// Class-level constraint violation.
		{
			name: "auth at_least_one violated when neither service nor dst_endpoint set",
			event: mutateAuth(func(a *iam.Authentication) {
				a.Service = nil
				a.DstEndpoint = nil
			}),
			wantRule:  "constraint",
			wantField: "service,dst_endpoint",
		},
	}

	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			err := r.event.Validate()
			if r.wantNil {
				if err != nil {
					t.Errorf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatal("Validate() = nil, want a *ValidationError")
			}
			var verr *ocsf.ValidationError
			if !errors.As(err, &verr) {
				t.Fatalf("Validate() returned %T, want *ValidationError", err)
			}
			if r.wantRule != "" && verr.Rule != r.wantRule {
				t.Errorf("Rule = %q, want %q (err=%v)", verr.Rule, r.wantRule, err)
			}
			if r.wantField != "" && verr.Field != r.wantField {
				t.Errorf("Field = %q, want %q (err=%v)", verr.Field, r.wantField, err)
			}
		})
	}
}

func TestValidate_PackageLevel(t *testing.T) {
	// The package-level Validate shim should behave the same as
	// the method.
	a := iam.Authentication{}
	mErr := a.Validate()
	pErr := ocsf.Validate(a)
	if (mErr == nil) != (pErr == nil) {
		t.Errorf("method err=%v, package err=%v — should match", mErr, pErr)
	}
}

func TestValidate_NilEventReturnsValidationError(t *testing.T) {
	err := ocsf.Validate(nil)
	if err == nil {
		t.Fatal("Validate(nil) = nil, want *ValidationError")
	}
	var verr *ocsf.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("Validate(nil) = %T, want *ValidationError", err)
	}
	if verr.Reason != "event is nil" {
		t.Errorf("Reason = %q, want \"event is nil\"", verr.Reason)
	}
}

func TestValidate_BaseEventReturnsNil(t *testing.T) {
	// BaseEvent is the forward-compat fallback — no schema rules
	// known, so Validate always succeeds.
	b := &ocsf.BaseEvent{}
	if err := b.Validate(); err != nil {
		t.Errorf("BaseEvent.Validate() = %v, want nil", err)
	}
	// Also via the package-level shim and via the Event
	// interface.
	if err := ocsf.Validate(b); err != nil {
		t.Errorf("ocsf.Validate(BaseEvent) = %v, want nil", err)
	}
}
