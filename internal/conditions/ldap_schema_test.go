// SPDX-FileCopyrightText: 2026 Zenauth Ltd.
// SPDX-License-Identifier: Apache-2.0

package conditions_test

import (
	"testing"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"github.com/stretchr/testify/require"

	"github.com/cerbos/cerbos/internal/conditions"
	customtypes "github.com/cerbos/cerbos/internal/conditions/types"
)

func ldapSchemaGlobals(unknown string) map[string]any {
	schema := map[string]any{
		"attributeTypes": []any{
			map[string]any{"oid": "2.5.4.11", "names": []any{"ou", "organizationalUnitName"}, "match": "ignoreCase"},
			map[string]any{"oid": "0.9.2342.19200300.100.1.25", "names": []any{"dc"}, "match": "ignoreCase"},
			map[string]any{"oid": "1.2.3.4", "names": []any{"customName", "cuName"}, "match": "ignoreCase"},
			map[string]any{"oid": "1.2.3.5", "names": []any{"customExact"}, "match": "exact"},
			map[string]any{"oid": "1.2.3.6", "names": []any{"customOctets"}, "match": "octets"},
		},
	}
	if unknown != "" {
		schema["unknownAttributeTypes"] = unknown
	}

	return map[string]any{customtypes.LDAPSchemaGlobal: schema}
}

// evalBoth evaluates expr the way the rule table does and the way the query
// planner does, and requires both to agree.
func evalBoth(t *testing.T, expr string, globals map[string]any) (ref.Val, error) {
	t.Helper()

	vars := map[string]any{
		conditions.CELGlobalsIdent:  customtypes.VariablesMap(globals),
		conditions.CELGlobalsAbbrev: customtypes.VariablesMap(globals),
	}

	ast, iss := conditions.Compile(expr)
	require.NoError(t, iss.Err())

	prg, err := conditions.StdEnv.Program(ast, cel.EvalOptions(cel.OptOptimize), cel.CustomDecorator(conditions.CacheFriendlyTimeDecorator()))
	require.NoError(t, err)
	cached, _, cachedErr := prg.Eval(vars)

	planned, _, plannedErr := conditions.ContextEval(t.Context(), conditions.StdEnv, ast.NativeRep(), vars, conditions.Now())

	require.Equal(t, cachedErr == nil, plannedErr == nil, "rule table: %v, planner: %v", cachedErr, plannedErr)
	if cachedErr == nil {
		require.Equal(t, cached.Value(), planned.Value())
	}

	return cached, cachedErr
}

func TestLDAPSchemaFromGlobals(t *testing.T) {
	testCases := []struct {
		expr    string
		unknown string
		builtin bool
		wantErr bool
	}{
		// names, OIDs and matching rules come from the schema
		{expr: `ldapDN("customName=A,dc=test") == ldapDN("CUNAME=a,dc=test")`},
		{expr: `ldapDN("customName=A,dc=test") == ldapDN("1.2.3.4=a,dc=test")`},
		{expr: `ldapDN("customExact=A,dc=test") != ldapDN("customExact=a,dc=test")`},
		{expr: `ldapDN("customExact=A  b,dc=test") == ldapDN("customExact=A b,dc=test")`},
		{expr: `ldapDN("customOctets=A  b,dc=test") != ldapDN("customOctets=A b,dc=test")`},
		{expr: `ldapDN("ou=A,dc=test").matchSubtree(ldapDN("customName=x,organizationalUnitName=a,DC=Test"))`},
		// without the schema the same names are unknown: compared by name, values keep their case
		{expr: `ldapDN("customName=A,dc=test") != ldapDN("cuName=A,dc=test")`, builtin: true},
		{expr: `ldapDN("customName=A,dc=test") != ldapDN("customName=a,dc=test")`, builtin: true},
		// the schema replaces the built-in attribute types
		{expr: `ldapDN("cn=A,dc=test") != ldapDN("commonName=A,dc=test")`},
		{expr: `ldapDN("cn=A,dc=test") != ldapDN("cn=a,dc=test")`},
		{expr: `ldapDN("cn=A,dc=test") == ldapDN("commonName=a,dc=test")`, builtin: true},
		// a DN that is a literal is not folded with the built-in schema
		{expr: `ldapDN("customName=A,dc=test") == ldapDN("customName=" + "a,dc=test")`},
		// unknownAttributeTypes
		{expr: `ldapDN("other=A,dc=test") != ldapDN("other=a,dc=test")`, unknown: "exact"},
		{expr: `ldapDN("other=A,dc=test") == ldapDN("other=a,dc=test")`, unknown: "ignoreCase"},
		{expr: `ldapDN("other=A,dc=test") == ldapDN("other=A,dc=test")`, unknown: "reject", wantErr: true},
		{expr: `ldapDN("customName=A,dc=test") == ldapDN("cuName=a,dc=test")`, unknown: "reject"},
		{expr: `ldapDN("customName=A,dc=test") == ldapDN("cuName=a,dc=test")`, unknown: "bogus", wantErr: true},
		// the list keeps the schema for the strings its methods parse
		{expr: `ldapDNList(["customName=A,dc=test"]).hasDN("cuName=a,dc=test")`},
		{expr: `ldapDNList(["customName=A,dc=test"]).anyIn(["1.2.3.4=a,dc=test"])`},
		{expr: `ldapDNList(["customName=x,ou=A,dc=test"]).allDescendentOf("organizationalUnitName=a,dc=test")`},
		{expr: `ldapDNList(["other=A,dc=test"]).hasDN("other=A,dc=test")`, unknown: "reject", wantErr: true},
		{expr: `ldapDNList(["customName=A,dc=test"]).hasDN("other=A,dc=test")`, unknown: "reject", wantErr: true},
		// inside a comprehension
		{expr: `["customName=A,dc=test", "cuName=b,dc=test"].exists(d, ldapDN(d) == ldapDN("1.2.3.4=B,dc=test"))`},
	}

	for _, tc := range testCases {
		t.Run(tc.expr, func(t *testing.T) {
			var globals map[string]any
			if !tc.builtin {
				globals = ldapSchemaGlobals(tc.unknown)
			}

			have, err := evalBoth(t, tc.expr, globals)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, true, have.Value())
		})
	}
}

// The attribute types of UCS with an EQUALITY rule other than caseIgnoreMatch,
// as univention-generate-cerbos-ldap-schema emits them.
func TestLDAPSchemaMatchingRules(t *testing.T) {
	globals := map[string]any{customtypes.LDAPSchemaGlobal: map[string]any{
		"unknownAttributeTypes": "reject",
		"attributeTypes": []any{
			map[string]any{"oid": "2.5.4.3", "names": []any{"cn", "commonName"}, "match": "ignoreCase"},
			map[string]any{"oid": "0.9.2342.19200300.100.1.25", "names": []any{"dc", "domainComponent"}, "match": "ignoreCase"},
			map[string]any{"oid": "1.3.6.1.4.1.10176.4204.1.1", "names": []any{"univentionAppID"}, "match": "exact"},
			map[string]any{"oid": "1.3.6.1.4.1.5322.10.1.1", "names": []any{"krb5PrincipalName"}, "match": "octets"},
			map[string]any{"oid": "1.3.6.1.4.1.10176.1003.3", "names": []any{"univentionObjectIdentifier"}, "match": "ignoreCase"},
		},
	}}

	for _, expr := range []string{
		`ldapDN("univentionAppID=Foo,dc=com") != ldapDN("UNIVENTIONAPPID=foo,dc=com")`,
		`ldapDN("univentionAppID=Foo,dc=com") == ldapDN("UNIVENTIONAPPID=Foo,DC=COM")`,
		`ldapDN("univentionAppID=Foo,dc=com") == ldapDN("1.3.6.1.4.1.10176.4204.1.1=Foo,dc=com")`,
		`ldapDN("univentionAppID=Foo,dc=com") != ldapDN("1.3.6.1.4.1.10176.4204.1.1=foo,dc=com")`,
		`ldapDN("krb5PrincipalName=host/ucs.example.com@EXAMPLE.COM,cn=kerberos,dc=com") == ldapDN("KRB5PRINCIPALNAME=host/ucs.example.com@EXAMPLE.COM,CN=Kerberos,dc=com")`,
		`ldapDN("krb5PrincipalName=host/ucs.example.com@EXAMPLE.COM,cn=kerberos,dc=com") == ldapDN("1.3.6.1.4.1.5322.10.1.1=host/ucs.example.com@EXAMPLE.COM,cn=kerberos,dc=com")`,
		`ldapDN("krb5PrincipalName=host/ucs.example.com@EXAMPLE.COM,cn=kerberos,dc=com") != ldapDN("krb5PrincipalName=host/ucs.example.com@example.com,cn=kerberos,dc=com")`,
		`ldapDN("krb5PrincipalName=a  b,dc=com") != ldapDN("krb5PrincipalName=a b,dc=com")`,
		`ldapDN("krb5PrincipalName=a\u00ADb,dc=com") != ldapDN("krb5PrincipalName=ab,dc=com")`,
		`ldapDN("univentionObjectIdentifier=0f8b2c4e-6a1d-4e3f-9b7a-2c5d8e1f4a6b,dc=com") == ldapDN("1.3.6.1.4.1.10176.1003.3=0F8B2C4E-6A1D-4E3F-9B7A-2C5D8E1F4A6B,dc=com")`,
	} {
		t.Run(expr, func(t *testing.T) {
			have, err := evalBoth(t, expr, globals)
			require.NoError(t, err)
			require.Equal(t, true, have.Value())
		})
	}
}

func TestValidateLDAPSchema(t *testing.T) {
	require.NoError(t, customtypes.ValidateLDAPSchema(nil))
	require.NoError(t, customtypes.ValidateLDAPSchema(map[string]any{"environment": "staging"}))
	require.NoError(t, customtypes.ValidateLDAPSchema(ldapSchemaGlobals("reject")))
}

// The configuration file decodes nested maps to map[any]any, a test suite to
// maps with string keys.
func TestLDAPSchemaFromConfigFile(t *testing.T) {
	globals := map[string]any{customtypes.LDAPSchemaGlobal: map[any]any{
		"unknownAttributeTypes": "reject",
		"attributeTypes": []any{
			map[any]any{"oid": "0.9.2342.19200300.100.1.25", "names": []any{"dc"}, "match": "ignoreCase"},
			map[any]any{"oid": "1.2.3.4", "names": []any{"customName", "cuName"}, "match": "ignoreCase"},
		},
	}}

	have, err := evalBoth(t, `ldapDN("customName=A,dc=test") == ldapDN("CUNAME=a,dc=test")`, globals)
	require.NoError(t, err)
	require.Equal(t, true, have.Value())
}

func TestLDAPSchemaInvalid(t *testing.T) {
	testCases := map[string]any{
		"not a map":         "x",
		"unknown key":       map[string]any{"attributetypes": []any{}},
		"unknown entry key": map[string]any{"attributeTypes": []any{map[string]any{"oid": "1.2.3", "name": []any{"a"}, "match": "exact"}}},
		"oid is a name":     map[string]any{"attributeTypes": []any{map[string]any{"oid": "cn", "match": "exact"}}},
		"no match":          map[string]any{"attributeTypes": []any{map[string]any{"oid": "1.2.3"}}},
		"bad match":         map[string]any{"attributeTypes": []any{map[string]any{"oid": "1.2.3", "match": "caseIgnoreMatch"}}},
		"oid twice":         map[string]any{"attributeTypes": []any{map[string]any{"oid": "1.2.3", "match": "exact"}, map[string]any{"oid": "1.2.3", "match": "exact"}}},
		"name is an OID":    map[string]any{"attributeTypes": []any{map[string]any{"oid": "1.2.3", "names": []any{"1.2.4"}, "match": "exact"}}},
		"name of two":       map[string]any{"attributeTypes": []any{map[string]any{"oid": "1.2.3", "names": []any{"a"}, "match": "exact"}, map[string]any{"oid": "1.2.4", "names": []any{"A"}, "match": "exact"}}},
		"names no list":     map[string]any{"attributeTypes": []any{map[string]any{"oid": "1.2.3", "names": "a", "match": "exact"}}},
		"types no list":     map[string]any{"attributeTypes": map[string]any{}},
		"entry no map":      map[string]any{"attributeTypes": []any{"cn"}},
		"unknown no enum":   map[string]any{"unknownAttributeTypes": true},
	}

	for name, schema := range testCases {
		t.Run(name, func(t *testing.T) {
			globals := map[string]any{customtypes.LDAPSchemaGlobal: schema}

			_, err := evalBoth(t, `ldapDN("dc=test") == ldapDN("dc=test")`, globals)
			require.ErrorContains(t, err, "invalid ldapSchema global")

			require.ErrorContains(t, customtypes.ValidateLDAPSchema(globals), "ldapSchema: ")
		})
	}
}

// The query planner evaluates with unknown resource attributes. ldapDN() must
// pass the unknown on, or the plan for every policy that uses it fails.
func TestLDAPSchemaUnknownArgument(t *testing.T) {
	globals := customtypes.VariablesMap(ldapSchemaGlobals(""))
	vars, err := cel.PartialVars(map[string]any{
		conditions.CELGlobalsIdent:  globals,
		conditions.CELGlobalsAbbrev: globals,
	}, cel.AttributePattern(conditions.CELResourceAbbrev))
	require.NoError(t, err)

	for _, expr := range []string{
		`ldapDN("ou=a,dc=test").matchSubtree(ldapDN(R.attr.dn))`,
		`ldapDNList(R.attr.dns).hasDN("ou=a,dc=test")`,
	} {
		t.Run(expr, func(t *testing.T) {
			ast, iss := conditions.Compile(expr)
			require.NoError(t, iss.Err())

			have, _, err := conditions.ContextEval(t.Context(), conditions.StdEnv, ast.NativeRep(), vars, conditions.Now(), cel.EvalOptions(cel.OptPartialEval, cel.OptTrackState))
			require.NoError(t, err)
			require.True(t, types.IsUnknown(have), "got %v", have)
		})
	}
}
