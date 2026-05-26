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

// Authentication has constraints: {at_least_one: [service, dst_endpoint]}.
// Tests below populate the required fields (cloud, metadata,
// osint, user) plus an in-range activity_id so we get past
// required-field and enum gates before exercising the
// constraint check.
func authPassingPriorGates() iam.Authentication {
	return iam.Authentication{
		Cloud:      &objects.Cloud{Provider: "aws"},
		Metadata:   &objects.Metadata{Product: &objects.Product{Name: "p"}, Version: "1.3.0"},
		Osint:      []objects.Osint{{}},
		User:       &objects.User{Name: "alice"},
		ActivityID: 1, // Logon — in range
	}
}

func TestGeneratedValidate_AtLeastOne_NeitherSetFails(t *testing.T) {
	a := authPassingPriorGates()
	// Service and DstEndpoint both nil — at_least_one violated.
	err := a.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want constraint violation for at_least_one")
	}
	var verr *ocsf.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("Validate() returned %T, want *ValidationError", err)
	}
	if verr.Rule != "constraint" {
		t.Errorf("ValidationError.Rule = %q, want constraint", verr.Rule)
	}
	if verr.Field != "service,dst_endpoint" {
		t.Errorf("ValidationError.Field = %q, want service,dst_endpoint", verr.Field)
	}
}

func TestGeneratedValidate_AtLeastOne_ServiceSetPasses(t *testing.T) {
	a := authPassingPriorGates()
	a.Service = &objects.Service{Name: "ldap"}
	if err := a.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil with service set", err)
	}
}

func TestGeneratedValidate_AtLeastOne_DstEndpointSetPasses(t *testing.T) {
	a := authPassingPriorGates()
	a.DstEndpoint = &objects.NetworkEndpoint{Hostname: "host.example.com"}
	if err := a.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil with dst_endpoint set", err)
	}
}

func TestGeneratedValidate_AtLeastOne_BothSetPasses(t *testing.T) {
	a := authPassingPriorGates()
	a.Service = &objects.Service{Name: "ldap"}
	a.DstEndpoint = &objects.NetworkEndpoint{Hostname: "host.example.com"}
	if err := a.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil with both fields set", err)
	}
}
