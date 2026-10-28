// SPDX-FileCopyrightText: 2026 Zenauth Ltd.
// SPDX-License-Identifier: Apache-2.0

package types

import (
	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
)

// ldapDNScopes are the scopes of matchScope(), with the receiver as the base.
var ldapDNScopes = map[string]func(LDAPHierarchy, ref.Val) ref.Val{
	"base":      ldapDNMatchBaseFn,
	"one":       ldapDNImmediateParentOfFn,
	"children":  ldapDNAncestorOfFn,
	"subtree":   ldapDNIsOrAncestorOfFn,
	"ancestors": ldapDNDescendentOfFn,
	"hierarchy": ldapDNOverlapsFn,
}

func ldapDNMatchScopeFn(base LDAPHierarchy, scope, dn ref.Val) ref.Val {
	s, ok := scope.(types.String)
	if !ok {
		return types.MaybeNoSuchOverloadErr(scope)
	}

	f, ok := ldapDNScopes[string(s)]
	if !ok {
		return types.NewErr("invalid LDAP scope %q", string(s))
	}

	return f(base, dn)
}

// LDAPScopeValidator rejects a policy whose matchScope() call has a literal
// scope that does not exist. Without it a misspelled scope is only an error
// when a request reaches the condition.
func LDAPScopeValidator() cel.ASTValidator {
	return ldapScopeValidator{}
}

type ldapScopeValidator struct{}

func (ldapScopeValidator) Name() string {
	return "cerbos.validator.ldap_scope"
}

func (ldapScopeValidator) Validate(_ *cel.Env, _ cel.ValidatorConfig, a *ast.AST, iss *cel.Issues) {
	for _, call := range ast.MatchDescendants(ast.NavigateAST(a), ast.FunctionMatcher(ldapDNMatchScope)) {
		args := call.AsCall().Args()
		if len(args) == 0 || args[0].Kind() != ast.LiteralKind {
			continue
		}

		scope, ok := args[0].AsLiteral().(types.String)
		if !ok {
			continue
		}

		if _, ok := ldapDNScopes[string(scope)]; !ok {
			iss.ReportErrorAtID(args[0].ID(), "invalid LDAP scope %q", string(scope))
		}
	}
}
