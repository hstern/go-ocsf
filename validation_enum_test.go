// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

package ocsf_test

import (
	"errors"
	"testing"

	ocsf "github.com/hstern/go-ocsf"
	"github.com/hstern/go-ocsf/events/iam"
	"github.com/hstern/go-ocsf/objects"
)

// fullyPopulatedAuth returns an Authentication with all
// required pointer / slice fields satisfied so Validate gets
// past the required-field gate and into the enum-membership
// and sibling-correspondence checks added by OCSF-17. Tests
// in this file shadow individual fields to provoke specific
// enum/correspondence failures.
func fullyPopulatedAuth() iam.Authentication {
	return iam.Authentication{
		Cloud:    &objects.Cloud{Provider: "aws"},
		Metadata: &objects.Metadata{Product: &objects.Product{Name: "p"}, Version: "1.3.0"},
		Osint:    []objects.Osint{{}},
		User:     &objects.User{Name: "alice"},
	}
}

func TestGeneratedValidate_EnumOutOfRange(t *testing.T) {
	a := fullyPopulatedAuth()
	// activity_id known values are 0..11, 99. 50 is outside.
	a.ActivityID = 50
	err := a.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want enum violation for activity_id=50")
	}
	var verr *ocsf.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("Validate() returned %T, want *ValidationError", err)
	}
	if verr.Rule != "enum" {
		t.Errorf("ValidationError.Rule = %q, want enum", verr.Rule)
	}
	if verr.Field != "activity_id" {
		t.Errorf("ValidationError.Field = %q, want activity_id", verr.Field)
	}
}

func TestGeneratedValidate_EnumKnownValuePasses(t *testing.T) {
	a := fullyPopulatedAuth()
	a.ActivityID = 1 // Logon — in range
	if err := a.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil for in-range activity_id=1 with empty activity_name", err)
	}
}

func TestGeneratedValidate_SiblingMismatch(t *testing.T) {
	a := fullyPopulatedAuth()
	a.ActivityID = 1 // Logon
	a.ActivityName = "Bogus"
	err := a.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want sibling-correspondence violation")
	}
	var verr *ocsf.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("Validate() returned %T, want *ValidationError", err)
	}
	if verr.Rule != "enum" {
		t.Errorf("ValidationError.Rule = %q, want enum", verr.Rule)
	}
	if verr.Field != "activity_name" {
		t.Errorf("ValidationError.Field = %q, want activity_name", verr.Field)
	}
}

func TestGeneratedValidate_SiblingMatchPasses(t *testing.T) {
	a := fullyPopulatedAuth()
	a.ActivityID = 2 // Logoff
	a.ActivityName = "Logoff"
	if err := a.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil for matching sibling pair", err)
	}
}

func TestGeneratedValidate_OtherSkipsSiblingCheck(t *testing.T) {
	a := fullyPopulatedAuth()
	a.ActivityID = 99 // "Other" — free-form sibling allowed by upstream convention.
	a.ActivityName = "Vendor-specific custom verb"
	if err := a.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil for id=99 (Other) with free-form sibling", err)
	}
}

func TestGeneratedValidate_EmptySiblingSkipsCorrespondence(t *testing.T) {
	a := fullyPopulatedAuth()
	a.ActivityID = 1 // Logon
	a.ActivityName = ""
	// Sibling not set → correspondence check should not fire.
	if err := a.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil when sibling string is empty", err)
	}
}
