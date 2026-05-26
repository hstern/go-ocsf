// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

package ocsf_test

import (
	"errors"
	"strings"
	"testing"

	ocsf "github.com/hstern/go-ocsf"
	"github.com/hstern/go-ocsf/events/iam"
	"github.com/hstern/go-ocsf/objects"
)

func TestValidationError_ErrorString(t *testing.T) {
	verr := &ocsf.ValidationError{
		ClassUID: 3002,
		Field:    "user",
		Rule:     "required",
		Reason:   "required field is missing",
	}
	got := verr.Error()
	wantParts := []string{"3002", "user", "required", "missing"}
	for _, p := range wantParts {
		if !strings.Contains(got, p) {
			t.Errorf("Error() = %q, missing %q", got, p)
		}
	}
}

func TestValidationError_NilReceiverDoesNotPanic(t *testing.T) {
	// Defensive: a nil *ValidationError shouldn't panic in
	// .Error() — callers that pass an error variable through
	// errors.As may end up dereferencing here.
	var verr *ocsf.ValidationError
	if got := verr.Error(); got != "<nil>" {
		t.Errorf("nil .Error() = %q, want <nil>", got)
	}
}

func TestValidationError_SatisfiesErrorAs(t *testing.T) {
	// errors.As / errors.Is integration.
	var err error = &ocsf.ValidationError{ClassUID: 1, Field: "x", Rule: "required"}
	var verr *ocsf.ValidationError
	if !errors.As(err, &verr) {
		t.Fatal("errors.As did not match *ValidationError")
	}
	if verr.ClassUID != 1 {
		t.Errorf("verr.ClassUID = %d, want 1", verr.ClassUID)
	}
}

func TestGeneratedValidate_AuthenticationMissingRequired(t *testing.T) {
	// Authentication requires user, metadata, cloud, osint
	// (via base_event + iam + cloud profile + osint profile
	// includes). Empty struct → Validate fails on the first
	// missing one.
	var a iam.Authentication
	err := a.Validate()
	if err == nil {
		t.Fatal("empty Authentication.Validate() returned nil, want a ValidationError")
	}
	var verr *ocsf.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("Validate() returned %T, want *ValidationError", err)
	}
	if verr.ClassUID != 3002 {
		t.Errorf("ValidationError.ClassUID = %d, want 3002", verr.ClassUID)
	}
	if verr.Rule != "required" {
		t.Errorf("ValidationError.Rule = %q, want required", verr.Rule)
	}
	// "First violation" rule: should be the alphabetically-first
	// required field that's empty — cloud (Cloud, Metadata,
	// Osint, User sorted alphabetically).
	if verr.Field != "cloud" {
		t.Errorf("ValidationError.Field = %q, want cloud (the first sorted required field)", verr.Field)
	}
}

func TestGeneratedValidate_PassesWhenRequiredArePresent(t *testing.T) {
	a := iam.Authentication{
		Cloud:    &objects.Cloud{Provider: "aws"},
		Metadata: &objects.Metadata{Product: &objects.Product{Name: "p"}, Version: "1.3.0"},
		Osint:    []objects.Osint{{}},
		User:     &objects.User{Name: "alice"},
		// Satisfies the at_least_one constraint added in OCSF-20.
		Service: &objects.Service{Name: "ldap"},
	}
	if err := a.Validate(); err != nil {
		t.Errorf("Authentication.Validate() = %v, want nil with all required fields set", err)
	}
}
