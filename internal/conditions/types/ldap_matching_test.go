// SPDX-FileCopyrightText: 2026 Univention GmbH
// SPDX-License-Identifier: AGPL-3.0-only

package types

import (
	"testing"

	"cel.dev/cel-go/common/types"
	"github.com/stretchr/testify/require"
)

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
