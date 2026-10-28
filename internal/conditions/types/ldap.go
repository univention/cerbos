// SPDX-FileCopyrightText: 2026 Univention GmbH
// SPDX-License-Identifier: AGPL-3.0-only

package types

import (
	"fmt"
	"reflect"
	"slices"

	ldapv3 "github.com/go-ldap/ldap/v3"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/decls"
	"cel.dev/cel-go/common/operators"
	"cel.dev/cel-go/common/overloads"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/common/types/traits"
)

const (
	ldapDNFn                = "ldapDN"
	ldapDNTypeName          = "cerbos.lib.ldap"
	ldapDNAncestorOf        = "ancestorOf"
	ldapDNCommonAncestors   = "commonAncestors"
	ldapDNDescendentOf      = "descendentOf"
	ldapDNImmediateChildOf  = "immediateChildOf"
	ldapDNImmediateParentOf = "immediateParentOf"
	ldapDNIsOrAncestorOf    = "isOrAncestorOf"
	ldapDNOverlaps          = "overlaps"
	ldapDNSiblingOf         = "siblingOf"

	ldapDNScopeBase     = "scopeBase"
	ldapDNScopeOneLevel = "scopeOneLevel"
	ldapDNScopeChildren = "scopeChildren"
	ldapDNScopeSubtree  = "scopeSubtree"

	ldapDNListFn              = "ldapDNList"
	ldapDNListTypeName        = "cerbos.lib.ldapList"
	ldapDNListContains        = "hasDN"
	ldapDNListAllIn           = "allIn"
	ldapDNListAnyIn           = "anyIn"
	ldapDNListSubsetOf        = "subsetOf"
	ldapDNListIntersects      = "intersects"
	ldapDNListEquals          = "equalsDN"
	ldapDNListAnyDescendentOf = "anyDescendentOf"
	ldapDNListAllDescendentOf = "allDescendentOf"
)

var (
	LDAPDNType = cel.ObjectType(
		ldapDNTypeName,
		traits.IndexerType,
		traits.SizerType,
		traits.ReceiverType,
	)

	LDAPDNFunc = cel.Function(
		ldapDNFn,
		cel.Overload(
			fmt.Sprintf("%s_string", ldapDNFn),
			[]*cel.Type{cel.StringType},
			LDAPDNType,
			cel.UnaryBinding(unaryLDAPDNFnImpl),
		),
	)

	LDAPDNDeclarations = []*decls.FunctionDecl{
		newFunction(ldapDNAncestorOf,
			decls.MemberOverload(
				"ldapDN_"+ldapDNAncestorOf,
				[]*types.Type{LDAPDNType, LDAPDNType},
				types.BoolType,
			),
		),

		newFunction(ldapDNCommonAncestors,
			decls.MemberOverload(
				"ldapDN_"+ldapDNCommonAncestors,
				[]*types.Type{LDAPDNType, LDAPDNType},
				LDAPDNType,
			),
		),

		newFunction(ldapDNDescendentOf,
			decls.MemberOverload(
				"ldapDN_"+ldapDNDescendentOf,
				[]*types.Type{LDAPDNType, LDAPDNType},
				types.BoolType,
			),
		),

		newFunction(ldapDNImmediateChildOf,
			decls.MemberOverload(
				"ldapDN_"+ldapDNImmediateChildOf,
				[]*types.Type{LDAPDNType, LDAPDNType},
				types.BoolType,
			),
		),

		newFunction(ldapDNImmediateParentOf,
			decls.MemberOverload(
				"ldapDN_"+ldapDNImmediateParentOf,
				[]*types.Type{LDAPDNType, LDAPDNType},
				types.BoolType,
			),
		),

		newFunction(ldapDNIsOrAncestorOf,
			decls.MemberOverload(
				"ldapDN_"+ldapDNIsOrAncestorOf,
				[]*types.Type{LDAPDNType, LDAPDNType},
				types.BoolType,
			),
		),

		newFunction(ldapDNOverlaps,
			decls.MemberOverload(
				"ldapDN_"+ldapDNOverlaps,
				[]*types.Type{LDAPDNType, LDAPDNType},
				types.BoolType,
			),
		),

		newFunction(ldapDNSiblingOf,
			decls.MemberOverload(
				"ldapDN_"+ldapDNSiblingOf,
				[]*types.Type{LDAPDNType, LDAPDNType},
				types.BoolType,
			),
		),

		newFunction(ldapDNScopeBase,
			decls.MemberOverload(
				"ldapDN_"+ldapDNScopeBase,
				[]*types.Type{LDAPDNType, LDAPDNType},
				types.BoolType,
			),
		),

		newFunction(ldapDNScopeOneLevel,
			decls.MemberOverload(
				"ldapDN_"+ldapDNScopeOneLevel,
				[]*types.Type{LDAPDNType, LDAPDNType},
				types.BoolType,
			),
		),

		newFunction(ldapDNScopeChildren,
			decls.MemberOverload(
				"ldapDN_"+ldapDNScopeChildren,
				[]*types.Type{LDAPDNType, LDAPDNType},
				types.BoolType,
			),
		),

		newFunction(ldapDNScopeSubtree,
			decls.MemberOverload(
				"ldapDN_"+ldapDNScopeSubtree,
				[]*types.Type{LDAPDNType, LDAPDNType},
				types.BoolType,
			),
		),

		newFunction(overloads.Size,
			decls.MemberOverload(fmt.Sprintf("%s_size", ldapDNFn),
				[]*types.Type{LDAPDNType},
				types.IntType,
			),
		),

		newFunction(operators.Index,
			decls.Overload(fmt.Sprintf("%s_index", ldapDNFn),
				[]*types.Type{LDAPDNType, types.IntType},
				types.StringType,
			),
		),
	}

	LDAPDNListType = cel.ObjectType(
		ldapDNListTypeName,
		traits.ReceiverType,
		traits.SizerType,
	)

	LDAPDNListFunc = cel.Function(
		ldapDNListFn,
		cel.Overload(
			fmt.Sprintf("%s_list", ldapDNListFn),
			[]*cel.Type{cel.ListType(cel.StringType)},
			LDAPDNListType,
			cel.UnaryBinding(unaryLDAPDNListFnImpl),
		),
	)

	LDAPDNListDeclarations = []*decls.FunctionDecl{
		newFunction(overloads.Size,
			decls.MemberOverload(fmt.Sprintf("%s_size", ldapDNListFn),
				[]*types.Type{LDAPDNListType},
				types.IntType,
			),
		),
		newFunction(ldapDNListContains,
			decls.MemberOverload(
				"ldapDNList_"+ldapDNListContains,
				[]*types.Type{LDAPDNListType, types.StringType},
				types.BoolType,
			),
		),
		newFunction(ldapDNListAllIn,
			decls.MemberOverload(
				"ldapDNList_"+ldapDNListAllIn,
				[]*types.Type{LDAPDNListType, types.NewListType(types.StringType)},
				types.BoolType,
			),
		),
		newFunction(ldapDNListAnyIn,
			decls.MemberOverload(
				"ldapDNList_"+ldapDNListAnyIn,
				[]*types.Type{LDAPDNListType, types.NewListType(types.StringType)},
				types.BoolType,
			),
		),
		newFunction(ldapDNListSubsetOf,
			decls.MemberOverload(
				"ldapDNList_"+ldapDNListSubsetOf,
				[]*types.Type{LDAPDNListType, types.NewListType(types.StringType)},
				types.BoolType,
			),
		),
		newFunction(ldapDNListIntersects,
			decls.MemberOverload(
				"ldapDNList_"+ldapDNListIntersects,
				[]*types.Type{LDAPDNListType, types.NewListType(types.StringType)},
				types.BoolType,
			),
		),
		newFunction(ldapDNListEquals,
			decls.MemberOverload(
				"ldapDNList_"+ldapDNListEquals,
				[]*types.Type{LDAPDNListType, types.NewListType(types.StringType)},
				types.BoolType,
			),
		),
		newFunction(ldapDNListAnyDescendentOf,
			decls.MemberOverload(
				"ldapDNList_"+ldapDNListAnyDescendentOf,
				[]*types.Type{LDAPDNListType, types.StringType},
				types.BoolType,
			),
		),
		newFunction(ldapDNListAllDescendentOf,
			decls.MemberOverload(
				"ldapDNList_"+ldapDNListAllDescendentOf,
				[]*types.Type{LDAPDNListType, types.StringType},
				types.BoolType,
			),
		),
	}

	ldapDNOneArgOverloads = map[string]func(LDAPHierarchy, ref.Val) ref.Val{
		ldapDNAncestorOf:        ldapDNAncestorOfFn,
		ldapDNCommonAncestors:   ldapDNCommonAncestorsFn,
		ldapDNDescendentOf:      ldapDNDescendentOfFn,
		ldapDNImmediateChildOf:  ldapDNImmediateChildOfFn,
		ldapDNImmediateParentOf: ldapDNImmediateParentOfFn,
		ldapDNIsOrAncestorOf:    ldapDNIsOrAncestorOfFn,
		ldapDNOverlaps:          ldapDNOverlapsFn,
		ldapDNSiblingOf:         ldapDNSiblingOfFn,

		// RFC 4511 search scopes, with the receiver as the search base.
		ldapDNScopeBase:     ldapDNScopeBaseFn,
		ldapDNScopeOneLevel: ldapDNImmediateParentOfFn,
		ldapDNScopeChildren: ldapDNAncestorOfFn,
		ldapDNScopeSubtree:  ldapDNIsOrAncestorOfFn,
	}
)

func unaryLDAPDNFnImpl(v ref.Val) ref.Val {
	switch dv := v.(type) {
	case LDAPHierarchy:
		return dv

	case types.String:
		dn, err := ldapv3.ParseDN(string(dv))
		if err != nil {
			return types.NewErr("invalid LDAP DN: %v", err)
		}

		h, err := newLDAPHierarchy(dn)
		if err != nil {
			return types.NewErr("invalid LDAP DN: %v", err)
		}

		return h

	default:
		return types.MaybeNoSuchOverloadErr(v)
	}
}

// LDAPHierarchy represents an LDAP Distinguished Name for hierarchy operations.
// Build it only with newLDAPHierarchy: a value without rdnKeys acts as the root
// DN, an ancestor of every DN.
type LDAPHierarchy struct {
	dn *ldapv3.DN
	// rdnKeys holds, for each RDN, its sorted and deduplicated normalized
	// type=value pairs. All comparisons use these instead of dn.
	rdnKeys [][]string
}

func newLDAPHierarchy(dn *ldapv3.DN) (LDAPHierarchy, error) {
	rdnKeys := make([][]string, len(dn.RDNs))
	for i, rdn := range dn.RDNs {
		keys := make([]string, len(rdn.Attributes))
		for j, attr := range rdn.Attributes {
			attrType := canonicalAttrType(attr.Type)
			value, err := prepareAttrValue(attrType, attr.Value)
			if err != nil {
				return LDAPHierarchy{}, fmt.Errorf("attribute %q: %w", attr.Type, err)
			}
			keys[j] = attrType + "=" + value
		}
		slices.Sort(keys)
		rdnKeys[i] = slices.Compact(keys)
	}

	return LDAPHierarchy{dn: dn, rdnKeys: rdnKeys}, nil
}

// ConvertToNative implements ref.Val.ConvertToNative.
func (l LDAPHierarchy) ConvertToNative(typeDesc reflect.Type) (any, error) {
	//nolint:exhaustive
	switch typeDesc.Kind() {
	case reflect.String:
		return l.dn.String(), nil
	case reflect.Interface:
		if reflect.TypeFor[LDAPHierarchy]().Implements(typeDesc) {
			return l, nil
		}
	}

	return nil, fmt.Errorf("unsupported native conversion from LDAP hierarchy to '%v'", typeDesc)
}

// ConvertToType implements ref.Val.ConvertToType.
func (l LDAPHierarchy) ConvertToType(typeVal ref.Type) ref.Val {
	switch typeVal {
	case types.StringType:
		return types.String(l.dn.String())
	case types.TypeType:
		return LDAPDNType
	}

	return types.NewErr("type conversion error from '%s' to '%s'", LDAPDNType, typeVal)
}

// Type implements ref.Val.Type.
func (l LDAPHierarchy) Type() ref.Type {
	return LDAPDNType
}

// Value implements ref.Val.Value.
func (l LDAPHierarchy) Value() any {
	return l.dn.String()
}

// Equal implements ref.Val.Equal.
func (l LDAPHierarchy) Equal(other ref.Val) ref.Val {
	otherDN, ok := other.(LDAPHierarchy)
	if !ok {
		return types.MaybeNoSuchOverloadErr(other)
	}

	return types.Bool(dnEqual(l, otherDN))
}

// Receive implements traits.Receiver.Receive.
func (l LDAPHierarchy) Receive(function, _ string, args []ref.Val) ref.Val {
	if len(args) == 1 {
		if f, ok := ldapDNOneArgOverloads[function]; ok {
			return f(l, args[0])
		}
	}

	return types.NoSuchOverloadErr()
}

// Get implements traits.Indexer.Get.
func (l LDAPHierarchy) Get(index ref.Val) ref.Val {
	i, ok := index.(types.Int)
	if !ok {
		return types.ValOrErr(index, "unsupported index type '%s'", index.Type())
	}

	idx := int(i)
	if idx < 0 || idx >= len(l.dn.RDNs) {
		return types.NewErr("index out of range")
	}

	return types.String(l.dn.RDNs[idx].String())
}

// Size implements traits.Sizer.Size.
func (l LDAPHierarchy) Size() ref.Val {
	return types.Int(len(l.dn.RDNs))
}

func ldapDNAncestorOfFn(h LDAPHierarchy, path ref.Val) ref.Val {
	child, err := toLDAP(path)
	if err != nil {
		return err
	}

	return types.Bool(isLDAPDNAncestorOf(h, child))
}

// ldapDNIsOrAncestorOfFn matches the scope of an LDAP subtree search: the base DN and everything below it.
func ldapDNIsOrAncestorOfFn(h LDAPHierarchy, path ref.Val) ref.Val {
	child, err := toLDAP(path)
	if err != nil {
		return err
	}

	return types.Bool(dnEqual(h, child) || isLDAPDNAncestorOf(h, child))
}

func ldapDNScopeBaseFn(h LDAPHierarchy, path ref.Val) ref.Val {
	other, err := toLDAP(path)
	if err != nil {
		return err
	}

	return types.Bool(dnEqual(h, other))
}

func ldapDNCommonAncestorsFn(h LDAPHierarchy, path ref.Val) ref.Val {
	other, err := toLDAP(path)
	if err != nil {
		return err
	}

	// Find the longest common suffix of RDNs (rightmost = ancestor in LDAP).
	minLen := min(len(other.rdnKeys), len(h.rdnKeys))

	commonLen := 0
	for commonLen < minLen {
		hIdx := len(h.rdnKeys) - 1 - commonLen
		otherIdx := len(other.rdnKeys) - 1 - commonLen
		if !slices.Equal(h.rdnKeys[hIdx], other.rdnKeys[otherIdx]) {
			break
		}
		commonLen++
	}

	start := len(h.rdnKeys) - commonLen

	return LDAPHierarchy{
		dn:      &ldapv3.DN{RDNs: slices.Clone(h.dn.RDNs[start:])},
		rdnKeys: h.rdnKeys[start:],
	}
}

func ldapDNDescendentOfFn(h LDAPHierarchy, path ref.Val) ref.Val {
	parent, err := toLDAP(path)
	if err != nil {
		return err
	}

	return types.Bool(isLDAPDNAncestorOf(parent, h))
}

func ldapDNImmediateParentOfFn(h LDAPHierarchy, path ref.Val) ref.Val {
	child, err := toLDAP(path)
	if err != nil {
		return err
	}

	if len(child.dn.RDNs) != len(h.dn.RDNs)+1 {
		return types.Bool(false)
	}

	return types.Bool(isLDAPDNAncestorOf(h, child))
}

func ldapDNImmediateChildOfFn(h LDAPHierarchy, path ref.Val) ref.Val {
	parent, err := toLDAP(path)
	if err != nil {
		return err
	}

	if len(h.dn.RDNs) != len(parent.dn.RDNs)+1 {
		return types.Bool(false)
	}

	return types.Bool(isLDAPDNAncestorOf(parent, h))
}

func ldapDNSiblingOfFn(h LDAPHierarchy, path ref.Val) ref.Val {
	other, err := toLDAP(path)
	if err != nil {
		return err
	}

	if len(h.rdnKeys) != len(other.rdnKeys) || len(h.rdnKeys) == 0 {
		return types.Bool(false)
	}

	// Siblings have different leftmost RDNs (a DN is not a sibling of itself)
	// and share the same parent path.
	return types.Bool(
		!slices.Equal(h.rdnKeys[0], other.rdnKeys[0]) &&
			rdnsEqual(h.rdnKeys[1:], other.rdnKeys[1:]),
	)
}

func ldapDNOverlapsFn(h LDAPHierarchy, path ref.Val) ref.Val {
	other, err := toLDAP(path)
	if err != nil {
		return err
	}

	return types.Bool(
		isLDAPDNAncestorOf(h, other) ||
			isLDAPDNAncestorOf(other, h),
	)
}

func toLDAP(v ref.Val) (LDAPHierarchy, ref.Val) {
	hv, ok := v.(LDAPHierarchy)
	if !ok {
		return LDAPHierarchy{}, types.MaybeNoSuchOverloadErr(v)
	}

	return hv, nil
}

func dnEqual(a, b LDAPHierarchy) bool {
	return rdnsEqual(a.rdnKeys, b.rdnKeys)
}

func rdnsEqual(a, b [][]string) bool {
	return slices.EqualFunc(a, b, slices.Equal[[]string])
}

// LDAPDNList is a collection of parsed LDAP Distinguished Names for use in
// set-oriented CEL policy expressions.
type LDAPDNList struct {
	dns []LDAPHierarchy
}

// ConvertToNative implements ref.Val.ConvertToNative.
func (l LDAPDNList) ConvertToNative(typeDesc reflect.Type) (any, error) {
	if typeDesc.Kind() == reflect.Interface && reflect.TypeFor[LDAPDNList]().Implements(typeDesc) {
		return l, nil
	}

	return nil, fmt.Errorf("unsupported native conversion from LDAPDNList to '%v'", typeDesc)
}

// ConvertToType implements ref.Val.ConvertToType.
func (l LDAPDNList) ConvertToType(typeVal ref.Type) ref.Val {
	if typeVal == types.TypeType {
		return LDAPDNListType
	}

	return types.NewErr("type conversion error from '%s' to '%s'", LDAPDNListType, typeVal)
}

// Equal implements ref.Val.Equal.
func (l LDAPDNList) Equal(other ref.Val) ref.Val {
	o, ok := other.(LDAPDNList)
	if !ok {
		return types.MaybeNoSuchOverloadErr(other)
	}

	return types.Bool(allDNsIn(l.dns, o.dns) && allDNsIn(o.dns, l.dns))
}

// Type implements ref.Val.Type.
func (l LDAPDNList) Type() ref.Type { return LDAPDNListType }

// Value implements ref.Val.Value.
func (l LDAPDNList) Value() any { return l }

// Size implements traits.Sizer.Size.
func (l LDAPDNList) Size() ref.Val { return types.Int(len(l.dns)) }

// Receive implements traits.Receiver.Receive.
func (l LDAPDNList) Receive(function, _ string, args []ref.Val) ref.Val {
	if len(args) != 1 {
		return types.NoSuchOverloadErr()
	}

	switch function {
	case ldapDNListContains:
		return ldapDNListContainsFn(l, args[0])
	case ldapDNListAllIn:
		return ldapDNListAllInFn(l, args[0])
	case ldapDNListAnyIn:
		return ldapDNListAnyInFn(l, args[0])
	case ldapDNListSubsetOf:
		return ldapDNListSubsetOfFn(l, args[0])
	case ldapDNListIntersects:
		return ldapDNListIntersectsFn(l, args[0])
	case ldapDNListEquals:
		return ldapDNListEqualsFn(l, args[0])
	case ldapDNListAnyDescendentOf:
		return ldapDNListAnyDescendentOfFn(l, args[0])
	case ldapDNListAllDescendentOf:
		return ldapDNListAllDescendentOfFn(l, args[0])
	}

	return types.NoSuchOverloadErr()
}

func unaryLDAPDNListFnImpl(v ref.Val) ref.Val {
	list, ok := v.(traits.Lister)
	if !ok {
		return types.MaybeNoSuchOverloadErr(v)
	}

	it := list.Iterator()
	var dns []LDAPHierarchy
	for it.HasNext() == types.True {
		elem := it.Next()
		s, ok := elem.(types.String)
		if !ok {
			return types.NewErr("ldapDNList: expected string elements, got '%s'", elem.Type())
		}
		h := unaryLDAPDNFnImpl(s)
		if types.IsError(h) {
			return h // fail closed on invalid DN
		}
		hv, ok := h.(LDAPHierarchy)
		if !ok {
			return types.MaybeNoSuchOverloadErr(h)
		}
		dns = append(dns, hv)
	}

	return LDAPDNList{dns: dns}
}

// parseDNStringList converts a CEL list of strings into []LDAPHierarchy.
// Fails closed: any unparseable DN is propagated as an error.
func parseDNStringList(v ref.Val) ([]LDAPHierarchy, ref.Val) {
	list, ok := v.(traits.Lister)
	if !ok {
		return nil, types.MaybeNoSuchOverloadErr(v)
	}

	it := list.Iterator()
	var dns []LDAPHierarchy
	for it.HasNext() == types.True {
		elem := it.Next()
		h := unaryLDAPDNFnImpl(elem)
		if types.IsError(h) {
			return nil, h
		}
		hv, ok := h.(LDAPHierarchy)
		if !ok {
			return nil, types.MaybeNoSuchOverloadErr(h)
		}
		dns = append(dns, hv)
	}

	return dns, nil
}

func ldapDNListContainsFn(l LDAPDNList, v ref.Val) ref.Val {
	h := unaryLDAPDNFnImpl(v)
	if types.IsError(h) {
		return h
	}

	target, ok := h.(LDAPHierarchy)
	if !ok {
		return types.MaybeNoSuchOverloadErr(h)
	}
	for _, d := range l.dns {
		if dnEqual(d, target) {
			return types.True
		}
	}

	return types.False
}

func ldapDNListAllInFn(l LDAPDNList, v ref.Val) ref.Val {
	allowed, err := parseDNStringList(v)
	if err != nil {
		return err
	}

	return types.Bool(allDNsIn(l.dns, allowed))
}

func allDNsIn(dns, allowed []LDAPHierarchy) bool {
	for _, d := range dns {
		if !slices.ContainsFunc(allowed, func(a LDAPHierarchy) bool { return dnEqual(d, a) }) {
			return false
		}
	}

	return true
}

func ldapDNListAnyInFn(l LDAPDNList, v ref.Val) ref.Val {
	allowed, err := parseDNStringList(v)
	if err != nil {
		return err
	}

	for _, d := range l.dns {
		for _, a := range allowed {
			if dnEqual(d, a) {
				return types.True
			}
		}
	}

	return types.False
}

// ldapDNListSubsetOfFn checks that every DN in l is contained in the allowed list.
// Semantically identical to allIn; subsetOf is the named alias for policy readability.
func ldapDNListSubsetOfFn(l LDAPDNList, v ref.Val) ref.Val {
	return ldapDNListAllInFn(l, v)
}

// ldapDNListIntersectsFn checks that at least one DN in l appears in the other list.
// Semantically identical to anyIn; intersects is the named alias for policy readability.
func ldapDNListIntersectsFn(l LDAPDNList, v ref.Val) ref.Val {
	return ldapDNListAnyInFn(l, v)
}

// ldapDNListEqualsFn checks that the two sets contain the same DNs (order-independent).
func ldapDNListEqualsFn(l LDAPDNList, v ref.Val) ref.Val {
	other, err := parseDNStringList(v)
	if err != nil {
		return err
	}

	return types.Bool(allDNsIn(l.dns, other) && allDNsIn(other, l.dns))
}

func ldapDNListAnyDescendentOfFn(l LDAPDNList, v ref.Val) ref.Val {
	base := unaryLDAPDNFnImpl(v)
	if types.IsError(base) {
		return base
	}

	baseDN, ok := base.(LDAPHierarchy)
	if !ok {
		return types.MaybeNoSuchOverloadErr(base)
	}
	for _, d := range l.dns {
		if isLDAPDNAncestorOf(baseDN, d) {
			return types.True
		}
	}

	return types.False
}

func ldapDNListAllDescendentOfFn(l LDAPDNList, v ref.Val) ref.Val {
	base := unaryLDAPDNFnImpl(v)
	if types.IsError(base) {
		return base
	}

	baseDN, ok := base.(LDAPHierarchy)
	if !ok {
		return types.MaybeNoSuchOverloadErr(base)
	}
	for _, d := range l.dns {
		if !isLDAPDNAncestorOf(baseDN, d) {
			return types.False
		}
	}

	return types.True
}

// isLDAPDNAncestorOf checks whether parent is a strict ancestor of child in the LDAP DN hierarchy.
// In LDAP, the rightmost RDNs are the ancestors (e.g., dc=com is above dc=example,dc=com).
// A DN is not considered an ancestor of itself.
func isLDAPDNAncestorOf(parent, child LDAPHierarchy) bool {
	offset := len(child.rdnKeys) - len(parent.rdnKeys)
	if offset <= 0 {
		return false
	}

	return rdnsEqual(parent.rdnKeys, child.rdnKeys[offset:])
}
