# LDAP DN functions

This fork adds two functions to the condition library of Cerbos.
The `ldapDN()` function parses an LDAP DN
and compares it with other DNs the way a directory does.
The `ldapDNList()` function holds a set of DNs.

```yaml
condition:
  match:
    expr: >
      ldapDN("cn=users,dc=example,dc=com").matchSubtree(ldapDN(R.attr.dn))
      && ldapDNList(P.attr.groups).hasDN("cn=Domain Admins,cn=groups,dc=example,dc=com")
```

For the discussion with the Cerbos maintainers,
see [Cerbos issue 3210](https://github.com/cerbos/cerbos/issues/3210).

## `ldapDN(string)`

The following table lists the methods of an `ldapDN` value.
They have the names of the `hierarchy()` methods.

| Method | Result |
| --- | --- |
| `a == b`, `a != b` | `a` and `b` are the same DN |
| `a.ancestorOf(b)` | `a` is above `b` |
| `a.descendentOf(b)` | `a` is below `b` |
| `a.isOrAncestorOf(b)` | `a` equals `b` or is above it |
| `a.immediateParentOf(b)` | `a` is one level above `b` |
| `a.immediateChildOf(b)` | `a` is one level below `b` |
| `a.siblingOf(b)` | `a` and `b` have the same parent and differ in the first RDN |
| `a.overlaps(b)` | `a` equals `b`, or one is above the other |
| `a.commonAncestors(b)` | Longest DN that is above or equal to both, as an `ldapDN` value. An error if there is none. |
| `a.size()` | Number of RDNs |
| `a[i]` | RDN `i` as a string, with the attribute type in lowercase |

### LDAP search scopes

The `matchScope()` method takes the scope as a string.
The receiver is the search base,
and the last argument is the entry to check.
Each scope also has a method of its own.

| Scope | `ldapsearch -s` | With `matchScope()` | Own method | Matches | Same as |
| --- | --- | --- | --- | --- | --- |
| Base | `base` | `base.matchScope("base", dn)` | `base.matchBase(dn)` | Base | `base == dn` |
| One level | `one` | `base.matchScope("one", dn)` | `base.matchOneLevel(dn)` | Direct children of the base | `base.immediateParentOf(dn)` |
| Children, or subordinate | `children` | `base.matchScope("children", dn)` | `base.matchChildren(dn)` | Everything below the base, without the base | `base.ancestorOf(dn)` |
| Subtree | `sub` | `base.matchScope("subtree", dn)` | `base.matchSubtree(dn)` | Base and everything below it | `base.isOrAncestorOf(dn)` |

The following two scopes have the same form
and aren't LDAP search scopes.

| With `matchScope()` | Own method | Matches | Same as |
| --- | --- | --- | --- |
| `base.matchScope("ancestors", dn)` | `base.matchAncestor(dn)` | Everything above the base, without the base | `base.descendentOf(dn)` |
| `base.matchScope("hierarchy", dn)` | `base.matchHierarchy(dn)` | Base, everything below it, and everything above it | `base.overlaps(dn)` |

The `matchScope()` method returns an error
for a scope that isn't in these tables.
If the scope is a literal, such as `"subtre"`,
Cerbos already rejects the policy when it compiles it.

## `ldapDNList(list<string>)`

The following table lists the methods of an `ldapDNList` value.

| Method | Result |
| --- | --- |
| `l.hasDN(dn)` | `dn` is in the set |
| `l.allIn(list)`, `l.subsetOf(list)` | Every DN of `l` is in `list` |
| `l.anyIn(list)`, `l.intersects(list)` | At least one DN of `l` is in `list` |
| `l.equalsDN(list)`, `l == other` | Both hold the same DNs |
| `l.anyDescendentOf(dn)` | At least one DN of `l` is below `dn` |
| `l.allDescendentOf(dn)` | Every DN of `l` is below `dn` |
| `l.size()` | Number of DNs as given, duplicates included |

The arguments are strings and lists of strings, not `ldapDN` values.
In the comparisons, order and duplicates don't matter.

An `ldapDNList` value isn't a CEL list.
It has no index operator,
and the `exists()`, `all()`, `filter()`, and `map()` macros don't accept it.
To iterate, use the list of strings and convert each element:

```cel
P.attr.groups.exists(g, ldapDN("ou=school1,dc=example,dc=com").matchSubtree(ldapDN(g)))
```

## DN comparison

The `ParseDN()` function of the `github.com/go-ldap/ldap/v3` module
parses and unescapes the string as RFC 4514 describes.
The comparison then works on attribute types and values, not on the string.
The following rules apply:

- An attribute type is a name or a numeric OID.
  Names compare without case.
  The names and the OID that the schema lists for one attribute type
  are the same type,
  so `cn=x`, `commonName=x`, and `2.5.4.3=x` are equal.
  An attribute type that the schema doesn't list compares by its name.
- A value compares by the matching rule of its attribute type.
  For the matching rules, see [LDAP schema](#ldap-schema).
- The attribute type and value pairs of a multi-valued RDN compare as a set,
  so `cn=a+uid=b` equals `uid=b+cn=a`.

The `ldapDN()` and `ldapDNList()` functions return an error, not `false`,
in the following cases:

- The string isn't a DN.
- The DN is empty. The empty DN is above every other DN.
- An attribute type is neither a name nor a numeric OID.
- A value has a character that RFC 4518 prohibits.
- The schema doesn't list an attribute type, and the schema rejects those.

With `engine.strictEvaluation: true`, an error denies the action.
Without that setting, Cerbos treats the condition as not met,
which skips a `DENY` rule.

CEL doesn't report an error inside `exists()`, `||`, or `&&`
when another element or operand already decides the result.
A list with one invalid DN and one matching DN matches.

## LDAP schema

The LDAP schema of a directory defines
which names belong to an attribute type and how its values compare.
Cerbos has no connection to the directory,
so it reads this part of the schema from a global with the name `ldapSchema`.

```yaml
engine:
  globals:
    ldapSchema:
      unknownAttributeTypes: reject
      attributeTypes:
        - oid: 2.5.4.3
          names: [cn, commonName]
          match: ignoreCase
        - oid: 1.3.6.1.4.1.10176.4204.1.1
          names: [univentionAppID]
          match: exact
```

The following table lists the keys of the `ldapSchema` global.

| Schema key | Value |
| --- | --- |
| `attributeTypes[].oid` | Numeric OID of the attribute type |
| `attributeTypes[].names` | Names of the attribute type, without case |
| `attributeTypes[].match` | `ignoreCase`, `exact`, or `octets` |
| `unknownAttributeTypes` | `exact`, `ignoreCase`, or `reject`. The default is `exact`. |

The following table lists how the values of an attribute type compare.

| `match` | Comparison |
| --- | --- |
| `ignoreCase` | RFC 4518 string preparation with case folding. `cn=Domain  Admins` equals `cn=domain admins`. |
| `exact` | RFC 4518 string preparation without case folding |
| `octets` | Byte for byte, without string preparation |

The string preparation removes control and zero-width characters,
applies NFKC normalization,
and treats a run of spaces as one space.

The `unknownAttributeTypes` key sets what happens
with an attribute type that the schema doesn't list:

- With `exact` and `ignoreCase`,
  the type compares by its name and the value by that rule.
- With `reject`, the `ldapDN()` function returns an error.

### Sources of the schema

The server reads the global from `engine.globals` in its configuration file.
It doesn't start with an invalid schema.

The `cerbos compile` command doesn't read the server configuration.
A test suite sets the global in `options.globals`:

```yaml
options:
  globals:
    ldapSchema:
      attributeTypes:
        - oid: 2.5.4.3
          names: [cn, commonName]
          match: ignoreCase
```

A policy doesn't pass the schema.
The `ldapDN()` function takes one argument.

### Built-in schema

Without the global, the schema in the
[`ldap_schema_default.yaml`](../internal/conditions/types/ldap_schema_default.yaml)
file applies.
It has 20 attribute types from RFC 4519 and RFC 4524 that are common in RDNs,
all with `ignoreCase`, and `unknownAttributeTypes: exact`.
It has no attribute types of UCS.

A configured schema replaces the built-in schema.
With the `ldapSchema` global set,
`cn` is a known attribute type only if the schema lists it.
The file has the format of the global,
so you can copy it as a starting point.

## Cache

Parsing and preparing a DN takes a few microseconds.
Cerbos keeps up to 1,024 parsed DNs in a cache, and only valid ones.
Hits and misses show up in the cache metrics with `kind="ldap_dn"`.

## Changes in upstream files

The following table lists the upstream files that the functions change.
Keep this list in mind when you move the patches to another upstream version.

| File | Change |
| --- | --- |
| `go.mod`, `go.sum` | `github.com/go-ldap/ldap/v3` as a direct dependency, with its two indirect dependencies |
| `internal/conditions/cerbos_lib.go` | Registers the two types, their functions, their methods, and the validator for the scope of `matchScope()` in `CompileOptions()`. Calls `DecorateLDAPCall()` in `cacheFriendlyTimeDecorate()` and `timeDecorator.decorate()`, which passes the schema from the globals to both functions. |
| `internal/conditions/cel.go` | Lists both functions in `unoptimizableFunctions`. Otherwise Cerbos evaluates `ldapDN("...")` with a literal argument when it loads the policy, with the built-in schema. |
| `internal/evaluator/conf.go` | Validates `engine.globals.ldapSchema` in `Conf.Validate()` |
| `internal/conditions/cerbos_lib_test.go` | Test cases for both functions in `TestCerbosLib()` |

## Files

The following table lists the files of the functions.

| File | Content |
| --- | --- |
| `internal/conditions/types/ldap.go` | CEL types, their methods, and the cache |
| `internal/conditions/types/ldap_schema.go` | Schema, its parser, and the lookup in the globals |
| `internal/conditions/types/ldap_schema_default.yaml` | Built-in schema |
| `internal/conditions/types/ldap_matching.go` | RFC 4518 string preparation |
| `internal/conditions/types/ldap_scope.go` | `matchScope()` method and the validator for its scope |
| `internal/conditions/ldap_schema_test.go` | Tests for a schema from the globals |
| `internal/conditions/ldap_scope_test.go` | Tests for the `matchScope()` method |
| `univention/tests/ldap-dn`, `univention/tests/ldap-hierarchy` | Policies and test suites that the `smoke-cerbos` job runs |

## Open points

- Nothing generates the `ldapSchema` global for a UCS system
  or writes it into the Cerbos configuration.
  A generator that reads `cn=Subschema` exists as a prototype
  in the `univention/dev/ucs` repository,
  on the `mmeschter/udm-authorization-cerbos-ldap-schema` branch.
- An attribute type whose value is a DN, such as `member`,
  compares as a string.
  The directory compares such values as DNs.
- The `docs/modules/policies/pages/conditions.adoc` file
  doesn't describe the functions.
- The cache size is a constant.
