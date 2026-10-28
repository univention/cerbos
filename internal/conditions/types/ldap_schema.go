// SPDX-FileCopyrightText: 2026 Zenauth Ltd.
// SPDX-License-Identifier: Apache-2.0

package types

import (
	_ "embed"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"cel.dev/cel-go/interpreter"
	"github.com/goccy/go-yaml"
	"google.golang.org/protobuf/types/known/structpb"
)

// LDAPSchemaGlobal is the key in the engine globals that holds the attribute
// types ldapDN() and ldapDNList() know.
const LDAPSchemaGlobal = "ldapSchema"

const (
	ldapSchemaAttributeTypes = "attributeTypes"
	ldapSchemaUnknown        = "unknownAttributeTypes"
	ldapSchemaOID            = "oid"
	ldapSchemaNames          = "names"
	ldapSchemaMatch          = "match"
	ldapSchemaTypeName       = "cerbos.lib.ldapSchema"
)

type ldapMatch uint8

const (
	ldapMatchIgnoreCase ldapMatch = iota
	ldapMatchExact
	ldapMatchOctets
)

var ldapMatchNames = map[string]ldapMatch{
	"ignoreCase": ldapMatchIgnoreCase,
	"exact":      ldapMatchExact,
	"octets":     ldapMatchOctets,
}

type ldapUnknown uint8

const (
	ldapUnknownExact ldapUnknown = iota
	ldapUnknownIgnoreCase
	ldapUnknownReject
)

var ldapUnknownNames = map[string]ldapUnknown{
	"exact":      ldapUnknownExact,
	"ignoreCase": ldapUnknownIgnoreCase,
	"reject":     ldapUnknownReject,
}

var errUnknownAttrType = errors.New("unknown attribute type")

// ldapSchema is the part of an LDAP schema that DN comparison needs. It is
// never modified after it is built.
type ldapSchema struct {
	oids    map[string]string    // lowercase name to OID
	match   map[string]ldapMatch // OID to EQUALITY rule
	unknown ldapUnknown
}

//go:embed ldap_schema_default.yaml
var defaultLDAPSchemaYAML []byte

var defaultLDAPSchema = func() *ldapSchema {
	var conf map[string]any
	if err := yaml.Unmarshal(defaultLDAPSchemaYAML, &conf); err != nil {
		panic(fmt.Errorf("ldap_schema_default.yaml: %w", err))
	}

	s, err := parseLDAPSchema(conf)
	if err != nil {
		panic(fmt.Errorf("ldap_schema_default.yaml: %w", err))
	}

	return s
}()

// canonicalAttrType returns the OID of an attribute type the schema knows, and
// the lowercase name of any other.
func (s *ldapSchema) canonicalAttrType(attrType string) string {
	t := strings.ToLower(attrType)
	// RFC 2253 allowed the prefix only before a numeric OID.
	if rest, ok := strings.CutPrefix(t, "oid."); ok && rest != "" && rest[0] >= '0' && rest[0] <= '9' {
		t = rest
	}

	if oid, ok := s.oids[t]; ok {
		return oid
	}

	return t
}

// matchFor returns how values of attrType, as returned by canonicalAttrType,
// are compared. Without the schema the EQUALITY rule of a type is unknown, and
// ignoring case for a case-exact type makes different entries compare equal.
func (s *ldapSchema) matchFor(attrType string) (ldapMatch, error) {
	if m, ok := s.match[attrType]; ok {
		return m, nil
	}

	switch s.unknown {
	case ldapUnknownIgnoreCase:
		return ldapMatchIgnoreCase, nil
	case ldapUnknownReject:
		return ldapMatchExact, errUnknownAttrType
	default:
		return ldapMatchExact, nil
	}
}

// prepareAttrValue applies the LDAP string preparation of RFC 4518 to an
// attribute value, so that values the directory considers equal compare equal.
func (s *ldapSchema) prepareAttrValue(attrType, value string) (string, error) {
	m, err := s.matchFor(attrType)
	if err != nil {
		return "", err
	}

	if m == ldapMatchOctets {
		return value, nil
	}

	foldCase := m == ldapMatchIgnoreCase
	if out, ok := prepareASCIIAttrValue(value, foldCase); ok {
		return out, nil
	}

	return prepareUnicodeAttrValue(value, foldCase)
}

// parseLDAPSchema builds a schema from the value of the ldapSchema global:
//
//	attributeTypes:
//	  - oid: 2.5.4.3
//	    names: [cn, commonName]
//	    match: ignoreCase            # ignoreCase, exact or octets
//	unknownAttributeTypes: exact     # exact, ignoreCase or reject
//
// The attribute types replace the built-in ones.
func parseLDAPSchema(v any) (*ldapSchema, error) {
	conf, ok := ldapNative(v).(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected a map, got %T", v)
	}

	s := &ldapSchema{oids: make(map[string]string), match: make(map[string]ldapMatch)}

	for key, val := range conf {
		switch key {
		case ldapSchemaUnknown:
			name, _ := val.(string)
			unknown, ok := ldapUnknownNames[name]
			if !ok {
				return nil, fmt.Errorf("%s: invalid value %v", ldapSchemaUnknown, val)
			}
			s.unknown = unknown
		case ldapSchemaAttributeTypes:
			entries, ok := val.([]any)
			if !ok {
				return nil, fmt.Errorf("%s: expected a list, got %T", ldapSchemaAttributeTypes, val)
			}
			for i, entry := range entries {
				if err := s.addAttributeType(entry); err != nil {
					return nil, fmt.Errorf("%s[%d]: %w", ldapSchemaAttributeTypes, i, err)
				}
			}
		default:
			return nil, fmt.Errorf("unknown key %q", key)
		}
	}

	return s, nil
}

// ValidateLDAPSchema checks the ldapSchema global, if the globals have one, so
// that the server refuses to start with a schema it would fail every ldapDN()
// call with.
func ValidateLDAPSchema(globals map[string]any) error {
	raw, ok := globals[LDAPSchemaGlobal]
	if !ok {
		return nil
	}

	if _, err := parseLDAPSchema(raw); err != nil {
		return fmt.Errorf("%s: %w", LDAPSchemaGlobal, err)
	}

	return nil
}

func (s *ldapSchema) addAttributeType(v any) error {
	entry, ok := v.(map[string]any)
	if !ok {
		return fmt.Errorf("expected a map, got %T", v)
	}

	for key := range entry {
		if key != ldapSchemaOID && key != ldapSchemaNames && key != ldapSchemaMatch {
			return fmt.Errorf("unknown key %q", key)
		}
	}

	oid, _ := entry[ldapSchemaOID].(string)
	if !isAttrType(oid) || oid[0] < '0' || oid[0] > '9' {
		return fmt.Errorf("%s: %q is not a numeric OID", ldapSchemaOID, oid)
	}

	if _, ok := s.match[oid]; ok {
		return fmt.Errorf("%s: %s is listed twice", ldapSchemaOID, oid)
	}

	matchName, _ := entry[ldapSchemaMatch].(string)
	match, ok := ldapMatchNames[matchName]
	if !ok {
		return fmt.Errorf("%s: invalid value %v", ldapSchemaMatch, entry[ldapSchemaMatch])
	}
	s.match[oid] = match

	names, ok := entry[ldapSchemaNames].([]any)
	if !ok && entry[ldapSchemaNames] != nil {
		return fmt.Errorf("%s: expected a list, got %T", ldapSchemaNames, entry[ldapSchemaNames])
	}

	for _, n := range names {
		name, _ := n.(string)
		name = strings.ToLower(name)
		if !isAttrType(name) || name[0] < 'a' || name[0] > 'z' {
			return fmt.Errorf("%s: %v is not an attribute type name", ldapSchemaNames, n)
		}

		if other, ok := s.oids[name]; ok && other != oid {
			return fmt.Errorf("%s: %s belongs to %s and %s", ldapSchemaNames, name, other, oid)
		}
		s.oids[name] = oid
	}

	return nil
}

// ldapNative converts the forms a global can have, depending on whether it
// comes from the configuration file, a test suite or CEL, to maps and slices.
func ldapNative(v any) any {
	switch t := v.(type) {
	case *structpb.Value:
		return t.AsInterface()
	case *structpb.Struct:
		return t.AsMap()
	case *structpb.ListValue:
		return t.AsSlice()
	case ref.Val:
		pb, err := t.ConvertToNative(reflect.TypeFor[*structpb.Value]())
		if err != nil {
			return v
		}

		return ldapNative(pb)
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = ldapNative(val)
		}

		return out
	case map[any]any:
		// The configuration file decodes nested maps to this type.
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[fmt.Sprint(k)] = ldapNative(val)
		}

		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = ldapNative(val)
		}

		return out
	default:
		return v
	}
}

type ldapSchemaEntry struct {
	// src keeps the parsed value alive, so that its address cannot be reused
	// for another value while it is the cache key.
	src    any
	schema *ldapSchema
	err    error
}

// ldapSchemas maps the address of an ldapSchema global to its parsed form.
// The engine passes the same value on every evaluation.
var ldapSchemas sync.Map

func ldapSchemaFromGlobals(globals any) (*ldapSchema, error) {
	var raw any
	switch g := globals.(type) {
	case nil:
		return defaultLDAPSchema, nil
	case Variables:
		if !g.IsSet(LDAPSchemaGlobal) {
			return defaultLDAPSchema, nil
		}
		raw, _ = g.Get(LDAPSchemaGlobal)
	case map[string]any:
		v, ok := g[LDAPSchemaGlobal]
		if !ok {
			return defaultLDAPSchema, nil
		}
		raw = v
	default:
		m, ok := ldapNative(globals).(map[string]any)
		if !ok {
			return nil, fmt.Errorf("unsupported globals type %T", globals)
		}

		return ldapSchemaFromGlobals(m)
	}

	rv := reflect.ValueOf(raw)
	if !rv.IsValid() || (rv.Kind() != reflect.Map && rv.Kind() != reflect.Pointer) || rv.IsNil() {
		return parseLDAPSchema(raw)
	}

	key := rv.Pointer()
	if e, ok := ldapSchemas.Load(key); ok {
		entry, _ := e.(*ldapSchemaEntry)
		return entry.schema, entry.err
	}

	schema, err := parseLDAPSchema(raw)
	ldapSchemas.Store(key, &ldapSchemaEntry{src: raw, schema: schema, err: err})

	return schema, err
}

// ldapSchemaVal carries a schema as the hidden second argument of ldapDN() and
// ldapDNList().
type ldapSchemaVal struct {
	schema *ldapSchema
}

var ldapSchemaType = cel.ObjectType(ldapSchemaTypeName)

func (ldapSchemaVal) ConvertToNative(typeDesc reflect.Type) (any, error) {
	return nil, fmt.Errorf("unsupported native conversion from LDAP schema to '%v'", typeDesc)
}

func (ldapSchemaVal) ConvertToType(typeVal ref.Type) ref.Val {
	return types.NewErr("type conversion error from '%s' to '%s'", ldapSchemaType, typeVal)
}

func (l ldapSchemaVal) Equal(other ref.Val) ref.Val {
	o, ok := other.(ldapSchemaVal)
	return types.Bool(ok && o.schema == l.schema)
}

func (ldapSchemaVal) Type() ref.Type { return ldapSchemaType }

func (l ldapSchemaVal) Value() any { return l.schema }

// ldapSchemaInterp resolves the schema from the globals of the activation.
type ldapSchemaInterp struct {
	globalsIdent string
}

func (*ldapSchemaInterp) ID() int64 { return 0 }

func (l *ldapSchemaInterp) Eval(activation interpreter.Activation) ref.Val {
	globals, _ := activation.ResolveName(l.globalsIdent)

	schema, err := ldapSchemaFromGlobals(globals)
	if err != nil {
		return types.NewErr("invalid %s global: %v", LDAPSchemaGlobal, err)
	}

	return ldapSchemaVal{schema: schema}
}

func (l *ldapSchemaInterp) Exec(frame *interpreter.ExecutionFrame) ref.Val {
	return l.Eval(frame)
}

// DecorateLDAPCall replaces a call to ldapDN() or ldapDNList() with one that
// takes the schema from the globals of the activation. A function binding gets
// only its arguments, so the schema is passed as a hidden second argument.
func DecorateLDAPCall(in interpreter.Interpretable, globalsIdent string) (interpreter.Interpretable, bool) {
	call, ok := in.(interpreter.InterpretableCall)
	if !ok || len(call.Args()) != 1 {
		return in, false
	}

	var build func(*ldapSchema, ref.Val) ref.Val
	switch call.Function() {
	case LDAPDNFn:
		build = ldapDNWithSchema
	case LDAPDNListFn:
		build = ldapDNListWithSchema
	default:
		return in, false
	}

	args := []interpreter.InterpretableV2{call.Args()[0], &ldapSchemaInterp{globalsIdent: globalsIdent}}

	return interpreter.NewCall(call.ID(), call.Function(), call.OverloadID(), args, func(values ...ref.Val) ref.Val {
		if len(values) != len(args) {
			return types.NoSuchOverloadErr()
		}

		schema, ok := values[1].(ldapSchemaVal)
		if !ok {
			return types.MaybeNoSuchOverloadErr(values[1])
		}

		return build(schema.schema, values[0])
	}), true
}
