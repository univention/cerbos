// SPDX-FileCopyrightText: 2026 Univention GmbH
// SPDX-License-Identifier: AGPL-3.0-only

package types

import (
	"math/rand/v2"
	"testing"

	"cel.dev/cel-go/common/types"
	"github.com/stretchr/testify/require"
)

func TestPrepareASCIIAttrValueMatchesUnicodePath(t *testing.T) {
	const alphabet = "aZz09 -_.\t\n\v\f\r\x00\x01\x08\x0e\x1f\x7f"
	caseIgnore := "2.5.4.3"
	caseExact := "1.3.6.1.4.1.10176.4204.1.1"
	require.False(t, caseExactAttrs[caseIgnore])
	require.True(t, caseExactAttrs[caseExact])

	rng := rand.New(rand.NewPCG(1, 2))
	values := []string{"", " ", "  a  ", "A\tB", "a\x00 \x01b", "Domain  Admins", "\x7f"}
	for range 20000 {
		b := make([]byte, rng.IntN(12))
		for i := range b {
			b[i] = alphabet[rng.IntN(len(alphabet))]
		}
		values = append(values, string(b))
	}

	for _, attrType := range []string{caseIgnore, caseExact} {
		for _, v := range values {
			want, err := prepareUnicodeAttrValue(attrType, v)
			require.NoError(t, err)
			have, ok := prepareASCIIAttrValue(v, !caseExactAttrs[attrType])
			require.True(t, ok)
			require.Equal(t, want, have, "attrType=%s value=%q", attrType, v)
		}
	}
}

func TestPrepareASCIIAttrValueRejectsNonASCII(t *testing.T) {
	for _, v := range []string{"Straße", "\u00ad", "a\xffb"} {
		_, ok := prepareASCIIAttrValue(v, true)
		require.False(t, ok, "value=%q", v)
	}
}

func TestLDAPDNCacheStoresOnlyValidDNs(t *testing.T) {
	valid := "cn=cached,dc=example,dc=com"
	invalid := "cn=\uE000,dc=com"

	first := unaryLDAPDNFnImpl(types.String(valid))
	require.True(t, ldapDNCache.Has(valid))
	require.Equal(t, types.True, first.Equal(unaryLDAPDNFnImpl(types.String(valid))))

	for range 2 {
		require.True(t, types.IsError(unaryLDAPDNFnImpl(types.String(invalid))))
	}
	require.False(t, ldapDNCache.Has(invalid))
}
