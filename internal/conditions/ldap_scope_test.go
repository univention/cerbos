// SPDX-FileCopyrightText: 2026 Zenauth Ltd.
// SPDX-License-Identifier: Apache-2.0

package conditions_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cerbos/cerbos/internal/conditions"
)

func TestLDAPMatchScope(t *testing.T) {
	testCases := []struct {
		expr string
		want bool
	}{
		{expr: `ldapDN("ou=a,dc=b").matchScope("base", ldapDN("OU=A,dc=b"))`, want: true},
		{expr: `ldapDN("ou=a,dc=b").matchScope("base", ldapDN("uid=x,ou=a,dc=b"))`},
		{expr: `ldapDN("ou=a,dc=b").matchScope("one", ldapDN("uid=x,ou=a,dc=b"))`, want: true},
		{expr: `ldapDN("ou=a,dc=b").matchScope("one", ldapDN("uid=x,cn=y,ou=a,dc=b"))`},
		{expr: `ldapDN("ou=a,dc=b").matchScope("one", ldapDN("ou=a,dc=b"))`},
		{expr: `ldapDN("ou=a,dc=b").matchScope("children", ldapDN("uid=x,cn=y,ou=a,dc=b"))`, want: true},
		{expr: `ldapDN("ou=a,dc=b").matchScope("children", ldapDN("ou=a,dc=b"))`},
		{expr: `ldapDN("ou=a,dc=b").matchScope("subtree", ldapDN("ou=a,dc=b"))`, want: true},
		{expr: `ldapDN("ou=a,dc=b").matchScope("subtree", ldapDN("uid=x,cn=y,ou=a,dc=b"))`, want: true},
		{expr: `ldapDN("ou=a,dc=b").matchScope("subtree", ldapDN("dc=b"))`},
		{expr: `ldapDN("ou=a,dc=b").matchScope("ancestors", ldapDN("dc=b"))`, want: true},
		{expr: `ldapDN("ou=a,dc=b").matchScope("ancestors", ldapDN("ou=a,dc=b"))`},
		{expr: `ldapDN("ou=a,dc=b").matchScope("hierarchy", ldapDN("dc=b"))`, want: true},
		{expr: `ldapDN("ou=a,dc=b").matchScope("hierarchy", ldapDN("uid=x,ou=a,dc=b"))`, want: true},
		{expr: `ldapDN("ou=a,dc=b").matchScope("hierarchy", ldapDN("ou=c,dc=b"))`},
		// a scope that is not a literal is checked when the condition is evaluated
		{expr: `["one", "subtree"].exists(s, ldapDN("ou=a,dc=b").matchScope(s, ldapDN("uid=x,cn=y,ou=a,dc=b")))`, want: true},
	}

	for _, tc := range testCases {
		t.Run(tc.expr, func(t *testing.T) {
			have, err := evalBoth(t, tc.expr, nil)
			require.NoError(t, err)
			require.Equal(t, tc.want, have.Value())
		})
	}
}

func TestLDAPMatchScopeInvalid(t *testing.T) {
	t.Run("literal", func(t *testing.T) {
		for _, expr := range []string{
			`ldapDN("ou=a,dc=b").matchScope("subtre", ldapDN("ou=a,dc=b"))`,
			`ldapDN("ou=a,dc=b").matchScope("Subtree", ldapDN("ou=a,dc=b"))`,
			`ldapDN("ou=a,dc=b").matchScope("", ldapDN("ou=a,dc=b"))`,
			`true || ldapDN("ou=a,dc=b").matchScope("sub", ldapDN("ou=a,dc=b"))`,
		} {
			_, iss := conditions.Compile(expr)
			require.ErrorContains(t, iss.Err(), "invalid LDAP scope", expr)
		}
	})

	t.Run("not a literal", func(t *testing.T) {
		_, err := evalBoth(t, `["subtre"].all(s, ldapDN("ou=a,dc=b").matchScope(s, ldapDN("ou=a,dc=b")))`, nil)
		require.ErrorContains(t, err, "invalid LDAP scope")
	})
}
