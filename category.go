// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

package ocsf

import "strconv"

// Category is the OCSF category_uid enum — the top-level
// grouping of event classes. Categories partition the schema:
// every concrete event class belongs to exactly one category,
// and its class_uid is computed as Category*1000 + the class's
// identifier within the category.
//
// The eight values below are stable for OCSF 1.x. Upstream
// has historically added categories without renumbering, so
// existing constants stay valid across schema versions; new
// categories appear as additional named constants in
// later library releases (e.g. CategoryUnmannedSystems
// arrived in 1.8.0).
type Category int

// String returns the OCSF category caption (e.g.
// CategoryNetworkActivity.String() == "Network Activity"). For
// values outside the known set, returns the decimal
// representation so log output stays useful for unmapped data.
func (c Category) String() string {
	switch c {
	case CategorySystemActivity:
		return "System Activity"
	case CategoryFindings:
		return "Findings"
	case CategoryIAM:
		return "Identity & Access Management"
	case CategoryNetworkActivity:
		return "Network Activity"
	case CategoryDiscovery:
		return "Discovery"
	case CategoryApplicationActivity:
		return "Application Activity"
	case CategoryRemediation:
		return "Remediation"
	case CategoryUnmannedSystems:
		return "Unmanned Systems"
	}
	return strconv.Itoa(int(c))
}

// Name returns the OCSF category_name (the snake_case
// identifier from upstream categories.json, e.g. "iam",
// "network"). For values outside the known set returns the
// empty string.
func (c Category) Name() string {
	switch c {
	case CategorySystemActivity:
		return "system"
	case CategoryFindings:
		return "findings"
	case CategoryIAM:
		return "iam"
	case CategoryNetworkActivity:
		return "network"
	case CategoryDiscovery:
		return "discovery"
	case CategoryApplicationActivity:
		return "application"
	case CategoryRemediation:
		return "remediation"
	case CategoryUnmannedSystems:
		return "unmanned_systems"
	}
	return ""
}

// OCSF category constants. Values match categories.json from
// the vendored schema and the wire-format category_uid.
const (
	// CategorySystemActivity is the System Activity category
	// (file, process, kernel, registry, scheduled job, ...).
	CategorySystemActivity Category = 1

	// CategoryFindings is the Findings category (detection
	// findings, vulnerability findings, compliance findings,
	// security findings).
	CategoryFindings Category = 2

	// CategoryIAM is the Identity & Access Management category
	// (authentication, authorization, account management,
	// session lifecycle).
	CategoryIAM Category = 3

	// CategoryNetworkActivity is the Network Activity category
	// (HTTP, DNS, SSH, TLS, file transfer, tunnel, ...).
	CategoryNetworkActivity Category = 4

	// CategoryDiscovery is the Discovery category (process /
	// file / device / inventory / patch query results, config
	// state).
	CategoryDiscovery Category = 5

	// CategoryApplicationActivity is the Application Activity
	// category (API, web resource, datastore, scan, application
	// lifecycle).
	CategoryApplicationActivity Category = 6

	// CategoryRemediation is the Remediation category (file,
	// process, network remediation activities).
	CategoryRemediation Category = 7

	// CategoryUnmannedSystems is the Unmanned Systems category
	// (UAV/UAS activity, mission planning, tracking). Added in
	// OCSF 1.8.0.
	CategoryUnmannedSystems Category = 8
)
