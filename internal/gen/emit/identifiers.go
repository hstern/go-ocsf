// Copyright 2026 The go-ocsf Authors
// SPDX-License-Identifier: Apache-2.0

package emit

import (
	"strings"
	"unicode"
)

// commonInitialisms lists snake-case segments that should be
// upper-cased in the Go identifier rather than Title-cased. The
// set is drawn from Go's stdlib naming (URL, ID, HTTP, ...) plus
// the OCSF-specific initialisms that appear across the schema
// (UID, CVE, CIDR, ...). When extending this list, prefer adding
// here over special-casing in the call site; this list is the
// single source of truth.
//
// The map is keyed by the all-lowercase form; the value is the
// upper-cased form used in Go identifiers.
var commonInitialisms = map[string]string{
	"acl":     "ACL",
	"api":     "API",
	"arn":     "ARN",
	"asn":     "ASN",
	"aws":     "AWS",
	"cidr":    "CIDR",
	"cors":    "CORS",
	"cpu":     "CPU",
	"csr":     "CSR",
	"css":     "CSS",
	"csv":     "CSV",
	"cve":     "CVE",
	"cvss":    "CVSS",
	"db":      "DB",
	"dhcp":    "DHCP",
	"dn":      "DN",
	"dns":     "DNS",
	"eof":     "EOF",
	"fips":    "FIPS",
	"ftp":     "FTP",
	"gcm":     "GCM",
	"gcp":     "GCP",
	"gpu":     "GPU",
	"guid":    "GUID",
	"hash":    "Hash",
	"hmac":    "HMAC",
	"http":    "HTTP",
	"https":   "HTTPS",
	"icmp":    "ICMP",
	"id":      "ID",
	"imap":    "IMAP",
	"io":      "IO",
	"ip":      "IP",
	"ipsec":   "IPSec",
	"irc":     "IRC",
	"json":    "JSON",
	"jwt":     "JWT",
	"kdc":     "KDC",
	"ldap":    "LDAP",
	"mac":     "MAC",
	"mfa":     "MFA",
	"mime":    "MIME",
	"mtu":     "MTU",
	"nat":     "NAT",
	"netbios": "NetBIOS",
	"ntp":     "NTP",
	"oauth":   "OAuth",
	"ocsf":    "OCSF",
	"os":      "OS",
	"oui":     "OUI",
	"pid":     "PID",
	"ppid":    "PPID",
	"pkcs":    "PKCS",
	"pop":     "POP",
	"qos":     "QoS",
	"ram":     "RAM",
	"rdp":     "RDP",
	"rfc":     "RFC",
	"rpc":     "RPC",
	"saml":    "SAML",
	"sha":     "SHA",
	"sid":     "SID",
	"smb":     "SMB",
	"smtp":    "SMTP",
	"snmp":    "SNMP",
	"sql":     "SQL",
	"ssh":     "SSH",
	"ssl":     "SSL",
	"sso":     "SSO",
	"tcp":     "TCP",
	"tid":     "TID",
	"tls":     "TLS",
	"tor":     "TOR",
	"ttl":     "TTL",
	"udp":     "UDP",
	"ui":      "UI",
	"uid":     "UID",
	"url":     "URL",
	"uri":     "URI",
	"utc":     "UTC",
	"utf":     "UTF",
	"utf8":    "UTF8",
	"uuid":    "UUID",
	"vlan":    "VLAN",
	"vm":      "VM",
	"vpn":     "VPN",
	"xml":     "XML",
	"xmpp":    "XMPP",
	"xss":     "XSS",
}

// goName converts an OCSF snake_case identifier to UpperCamelCase
// using the [commonInitialisms] table for known acronyms. Leading
// underscores (used by OCSF for UI-hidden objects like `_entity`)
// are stripped — they carry no privacy semantics in Go.
func goName(s string) string {
	s = strings.TrimLeft(s, "_")
	if s == "" {
		return ""
	}
	parts := strings.Split(s, "_")
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		if up, ok := commonInitialisms[strings.ToLower(p)]; ok {
			b.WriteString(up)
			continue
		}
		// Numeric segments stay as-is (e.g. utf8 stays "utf8" in
		// the lowercase form, but commonInitialisms upper-cases it
		// to UTF8 above).
		runes := []rune(p)
		runes[0] = unicode.ToUpper(runes[0])
		for i := 1; i < len(runes); i++ {
			runes[i] = unicode.ToLower(runes[i])
		}
		b.WriteString(string(runes))
	}
	return b.String()
}

// goFieldName is the identifier used for a struct field. Same as
// [goName] for now — the rule is uniform across object names and
// attribute names — but kept as a separate function so a future
// divergence (e.g. handling Go reserved words on fields) lands in
// one place.
func goFieldName(attr string) string {
	return goName(attr)
}
