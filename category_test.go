// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

package ocsf

import "testing"

func TestCategory_String(t *testing.T) {
	cases := []struct {
		in   Category
		want string
	}{
		{CategorySystemActivity, "System Activity"},
		{CategoryFindings, "Findings"},
		{CategoryIAM, "Identity & Access Management"},
		{CategoryNetworkActivity, "Network Activity"},
		{CategoryDiscovery, "Discovery"},
		{CategoryApplicationActivity, "Application Activity"},
		{CategoryRemediation, "Remediation"},
		{Category(0), "0"},
		{Category(99), "99"},
	}
	for _, c := range cases {
		if got := c.in.String(); got != c.want {
			t.Errorf("Category(%d).String() = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCategory_Name(t *testing.T) {
	cases := []struct {
		in   Category
		want string
	}{
		{CategorySystemActivity, "system"},
		{CategoryFindings, "findings"},
		{CategoryIAM, "iam"},
		{CategoryNetworkActivity, "network"},
		{CategoryDiscovery, "discovery"},
		{CategoryApplicationActivity, "application"},
		{CategoryRemediation, "remediation"},
		{Category(0), ""},
		{Category(42), ""},
	}
	for _, c := range cases {
		if got := c.in.Name(); got != c.want {
			t.Errorf("Category(%d).Name() = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCategoryConstantsMatchUpstream(t *testing.T) {
	// Sanity: the constants here MUST match the UID values in the
	// vendored categories.json. The codegen pipeline also reads
	// categories.json, so any drift between the constants and the
	// schema produces a class_uid mismatch (CategoryX*1000 vs.
	// generated OCSFCategoryUID()).
	cases := []struct {
		c       Category
		wantUID int
	}{
		{CategorySystemActivity, 1},
		{CategoryFindings, 2},
		{CategoryIAM, 3},
		{CategoryNetworkActivity, 4},
		{CategoryDiscovery, 5},
		{CategoryApplicationActivity, 6},
		{CategoryRemediation, 7},
	}
	for _, c := range cases {
		if int(c.c) != c.wantUID {
			t.Errorf("Category(%d) = %d, want %d", c.c, int(c.c), c.wantUID)
		}
	}
}
