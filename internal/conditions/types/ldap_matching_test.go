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

	rng := rand.New(rand.NewPCG(1, 2))
	values := []string{"", " ", "  a  ", "A\tB", "a\x00 \x01b", "Domain  Admins", "\x7f"}
	for range 20000 {
		b := make([]byte, rng.IntN(12))
		for i := range b {
			b[i] = alphabet[rng.IntN(len(alphabet))]
		}
		values = append(values, string(b))
	}

	for _, foldCase := range []bool{true, false} {
		for _, v := range values {
			want, err := prepareUnicodeAttrValue(v, foldCase)
			require.NoError(t, err)
			have, ok := prepareASCIIAttrValue(v, foldCase)
			require.True(t, ok)
			require.Equal(t, want, have, "foldCase=%v value=%q", foldCase, v)
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
	require.True(t, ldapDNCache.Has(ldapDNCacheKey{schema: defaultLDAPSchema, dn: valid}))
	require.Equal(t, types.True, first.Equal(unaryLDAPDNFnImpl(types.String(valid))))

	for range 2 {
		require.True(t, types.IsError(unaryLDAPDNFnImpl(types.String(invalid))))
	}
	require.False(t, ldapDNCache.Has(ldapDNCacheKey{schema: defaultLDAPSchema, dn: invalid}))
}
