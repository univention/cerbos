// SPDX-FileCopyrightText: 2026 Univention GmbH
// SPDX-License-Identifier: AGPL-3.0-only

package types

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// attrTypeOIDs maps the lowercase names of a subset of the attribute types
// defined in RFC 4519 and RFC 4524 to their OIDs, so that cn, commonName and
// 2.5.4.3 compare equal.
// A type that is not listed compares by its lowercase name, and its values
// keep their case.
var attrTypeOIDs = map[string]string{
	"cn":                     "2.5.4.3",
	"commonname":             "2.5.4.3",
	"sn":                     "2.5.4.4",
	"surname":                "2.5.4.4",
	"serialnumber":           "2.5.4.5",
	"c":                      "2.5.4.6",
	"countryname":            "2.5.4.6",
	"l":                      "2.5.4.7",
	"localityname":           "2.5.4.7",
	"st":                     "2.5.4.8",
	"stateorprovincename":    "2.5.4.8",
	"street":                 "2.5.4.9",
	"streetaddress":          "2.5.4.9",
	"o":                      "2.5.4.10",
	"organizationname":       "2.5.4.10",
	"ou":                     "2.5.4.11",
	"organizationalunitname": "2.5.4.11",
	"title":                  "2.5.4.12",
	"description":            "2.5.4.13",
	"postalcode":             "2.5.4.17",
	"name":                   "2.5.4.41",
	"givenname":              "2.5.4.42",
	"gn":                     "2.5.4.42",
	"initials":               "2.5.4.43",
	"generationqualifier":    "2.5.4.44",
	"dnqualifier":            "2.5.4.46",
	"uid":                    "0.9.2342.19200300.100.1.1",
	"userid":                 "0.9.2342.19200300.100.1.1",
	"mail":                   "0.9.2342.19200300.100.1.3",
	"rfc822mailbox":          "0.9.2342.19200300.100.1.3",
	"dc":                     "0.9.2342.19200300.100.1.25",
	"domaincomponent":        "0.9.2342.19200300.100.1.25",
}

// attrTypeOIDs, caseExactAttrs and octetStringAttrs are the attribute types of
// defaultLDAPSchema, used when the engine globals hold no ldapSchema.

// caseExactAttrs lists the attribute types in attrTypeOIDs, by OID, whose
// schema EQUALITY rule is caseExactMatch or caseExactIA5Match. Values of the
// other types in attrTypeOIDs are compared ignoring case, the LDAP norm. Only
// types used in RDNs matter: a missing entry makes entries that differ only in
// case compare equal.
var caseExactAttrs = map[string]bool{}

// octetStringAttrs lists the attribute types, as returned by canonicalAttrType,
// whose schema EQUALITY rule is octetStringMatch. Their values are compared
// byte for byte, without the RFC 4518 preparation.
var octetStringAttrs = map[string]bool{}

// mapToNothing holds the code points that RFC 4518 section 2.2 removes.
var mapToNothing = &unicode.RangeTable{
	R16: []unicode.Range16{
		{Lo: 0x0000, Hi: 0x0008, Stride: 1},
		{Lo: 0x000E, Hi: 0x001F, Stride: 1},
		{Lo: 0x007F, Hi: 0x0084, Stride: 1},
		{Lo: 0x0086, Hi: 0x009F, Stride: 1},
		{Lo: 0x00AD, Hi: 0x00AD, Stride: 1},
		{Lo: 0x034F, Hi: 0x034F, Stride: 1},
		{Lo: 0x06DD, Hi: 0x06DD, Stride: 1},
		{Lo: 0x070F, Hi: 0x070F, Stride: 1},
		{Lo: 0x1806, Hi: 0x1806, Stride: 1},
		{Lo: 0x180B, Hi: 0x180E, Stride: 1},
		{Lo: 0x200B, Hi: 0x200F, Stride: 1},
		{Lo: 0x202A, Hi: 0x202E, Stride: 1},
		{Lo: 0x2060, Hi: 0x2063, Stride: 1},
		{Lo: 0x206A, Hi: 0x206F, Stride: 1},
		{Lo: 0xFE00, Hi: 0xFE0F, Stride: 1},
		{Lo: 0xFEFF, Hi: 0xFEFF, Stride: 1},
		{Lo: 0xFFF9, Hi: 0xFFFC, Stride: 1},
	},
	R32: []unicode.Range32{
		{Lo: 0x1D173, Hi: 0x1D17A, Stride: 1},
		{Lo: 0xE0001, Hi: 0xE0001, Stride: 1},
		{Lo: 0xE0020, Hi: 0xE007F, Stride: 1},
	},
	LatinOffset: 5,
}

// isAttrType reports whether t, as returned by canonicalAttrType, is a name or
// a numeric OID as RFC 4512 section 1.4 defines them. Any other type could
// contain "=", and then two different type and value pairs give the same key.
func isAttrType(t string) bool {
	if t == "" {
		return false
	}

	if t[0] >= 'a' && t[0] <= 'z' {
		for i := 1; i < len(t); i++ {
			c := t[i]
			if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
				return false
			}
		}

		return true
	}

	if !strings.Contains(t, ".") {
		return false
	}

	for arc := range strings.SplitSeq(t, ".") {
		if arc == "" || (len(arc) > 1 && arc[0] == '0') {
			return false
		}
		for i := range len(arc) {
			if arc[i] < '0' || arc[i] > '9' {
				return false
			}
		}
	}

	return true
}

// prepareASCIIAttrValue gives the result of prepareUnicodeAttrValue for values
// that are pure ASCII, where NFKC changes nothing, case folding is lowercasing,
// and no character is prohibited. It reports false for any other value.
func prepareASCIIAttrValue(value string, foldCase bool) (string, bool) {
	b := make([]byte, 0, len(value))
	space := false
	for i := range len(value) {
		c := value[i]
		switch {
		case c >= utf8.RuneSelf:
			return "", false
		case c >= '\t' && c <= '\r', c == ' ':
			space = len(b) > 0
		case c < ' ', c == 0x7F:
		default:
			if space {
				b = append(b, ' ')
				space = false
			}
			if foldCase && c >= 'A' && c <= 'Z' {
				c += 'a' - 'A'
			}
			b = append(b, c)
		}
	}

	return string(b), true
}

func prepareUnicodeAttrValue(value string, foldCase bool) (string, error) {
	var b strings.Builder
	for _, r := range value {
		switch {
		case unicode.Is(mapToNothing, r):
		case r >= '\t' && r <= '\r', r == 0x85, unicode.Is(unicode.Z, r):
			b.WriteByte(' ')
		default:
			b.WriteRune(r)
		}
	}

	s := b.String()
	if !foldCase {
		s = norm.NFKC.String(s)
	} else {
		s = norm.NFKC.String(cases.Fold().String(s))
		// Folding again covers the characters that NFKC turns into uppercase
		// letters (U+2121 to "TEL"), as RFC 3454 table B.2 does.
		s = norm.NFKC.String(cases.Fold().String(s))
	}

	for _, r := range s {
		if isProhibited(r) {
			return "", fmt.Errorf("prohibited character %U", r)
		}
	}

	return strings.Join(strings.Fields(s), " "), nil
}

// isProhibited reports the code points that RFC 4518 section 2.4 rejects and
// that are left after mapping and normalization. utf8.RuneError also stands
// for invalid UTF-8.
func isProhibited(r rune) bool {
	return r == utf8.RuneError ||
		unicode.Is(unicode.Co, r) ||
		(r >= 0xFDD0 && r <= 0xFDEF) || r&0xFFFE == 0xFFFE ||
		!unicode.In(r, unicode.L, unicode.M, unicode.N, unicode.P, unicode.S, unicode.Z, unicode.C)
}
