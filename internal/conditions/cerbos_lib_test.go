// Copyright 2021-2026 Zenauth Ltd.
// SPDX-License-Identifier: Apache-2.0

package conditions_test

import (
	"log"
	"reflect"
	"testing"
	"time"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common"
	celast "cel.dev/cel-go/common/ast"
	"cel.dev/cel-go/common/decls"
	celtypes "cel.dev/cel-go/common/types"
	"cel.dev/cel-go/ext"
	"cel.dev/cel-go/parser"
	"github.com/stretchr/testify/require"

	enginev1 "github.com/cerbos/cerbos/api/genpb/cerbos/engine/v1"
	"github.com/cerbos/cerbos/internal/conditions"
	"github.com/cerbos/cerbos/internal/conditions/types"
)

func TestCerbosLib(t *testing.T) {
	testCases := []struct {
		expr    string
		wantErr bool
	}{
		{expr: `"192.168.0.5".inIPAddrRange("192.168.0.0/24") == true`},
		{expr: `"192.169.1.5".inIPAddrRange("192.168.0.0/24") == false`},
		{expr: `"test".inIPAddrRange("192.168.0.0/24") == false`, wantErr: true},
		{expr: `"2001:0db8:0000:0000:0000:0000:1000:0000".inIPAddrRange("2001:db8::/48") == true`},
		{expr: `"3001:0fff:0000:0000:0000:0000:0000:0000".inIPAddrRange("2001:db8::/48") == false`},
		{expr: `timestamp("2021-05-01T00:00:00.000Z").timeSince() > duration("1h")`},
		{expr: `has_intersection([1,2,3],[3,5])`},
		{expr: `has_intersection([1,2,3],[4,5]) == false`},
		{expr: `has_intersection(['1','2','3'],['3','5'])`},
		{expr: `hasIntersection([1,2,3],[3,5])`},
		{expr: `hasIntersection([1,2,3],[4,5]) == false`},
		{expr: `hasIntersection(['1','2','3'],['3','5'])`},
		{expr: `hasIntersection([1],[1.0,2])`},
		{expr: `hasIntersection([1.0,2],[1])`},
		{expr: `hasIntersection([1,2,3,4],[1.0,5,6,7])`},
		{expr: `hasIntersection([1.0,5,6,7],[1,2,3,4])`},
		{expr: `intersect([1,2,3],[2,3,5]) == [2,3]`},
		{expr: `intersect([1,2,3],[4,5]) == []`},
		{expr: `intersect(['1','2','3'],['3','5']) == ['3']`},
		{expr: `intersect([1],[1.0,2]) == [1]`},
		{expr: `intersect([1.0,2],[1]) == [1]`},
		{expr: `intersect([1,2,3,4],[1.0,5,6,7]) == [1]`},
		{expr: `intersect([1.0,5,6,7],[1,2,3,4]) == [1]`},
		{expr: `intersect([1],[1.0,2]) == [1.0]`},
		{expr: `intersect([1.0,2],[1]) == [1.0]`},
		{expr: `intersect([1,2,3,4],[1.0,5,6,7]) == [1.0]`},
		{expr: `intersect([1.0,5,6,7],[1,2,3,4]) == [1.0]`},
		{expr: `[1,2].is_subset([2,3]) == false`},
		{expr: `[[1],[2]].is_subset([[2],[3]]) == false`},
		{expr: `[1,2].is_subset([1,2])`},
		{expr: `[1,2].is_subset([1,2,3])`},
		{expr: `[[1],[2]].is_subset([[1],[2],[3]])`},
		{expr: `["1","2"].is_subset(["1","2","3"])`},
		{expr: `[1,1].is_subset([1])`},
		{expr: `[].is_subset([1])`},
		{expr: `[1,2].isSubset([2,3]) == false`},
		{expr: `[[1],[2]].isSubset([[2],[3]]) == false`},
		{expr: `[1,2].isSubset([1,2])`},
		{expr: `[1,2].isSubset([1,2,3])`},
		{expr: `[[1],[2]].isSubset([[1],[2],[3]])`},
		{expr: `["1","2"].isSubset(["1","2","3"])`},
		{expr: `[1,1].isSubset([1])`},
		{expr: `[].isSubset([1])`},
		{expr: `[1,2].isSubset([1.0,2])`},
		{expr: `[1,2].isSubset([1.0,2])`},
		{expr: `[1.0,2].isSubset([1,2])`},
		{expr: `[1,2,3,4].isSubset([1.0,2,3,4])`},
		{expr: `[1.0,2,3,4].isSubset([1,2,3,4])`},
		{expr: `[].except([1]) == []`},
		{expr: `[1].except([]) == [1]`},
		{expr: `[].except([]) == []`},
		{expr: `[1].except([1]) == []`},
		{expr: `[1].except([1,2,3]) == []`},
		{expr: `[1,3,5].except([2,4]) == [1,3,5]`},
		{expr: `[1,3,5].except([5,3]) == [1]`},
		{expr: `[1,2].except([1.0,2]) == []`},
		{expr: `[1.0,2].except([1,2]) == []`},
		{expr: `[1,2,3,4].except([1.0,2,3,4]) == []`},
		{expr: `[1.0,2,3,4].except([1,2,3,4]) == []`},
		{expr: `[1,2,3] + [3,5] == [1,2,3,3,5]`},
		{expr: `hierarchy("a.b.c.d") == hierarchy("a.b.c.d")`},
		{expr: `hierarchy("a.b.c.d") != hierarchy("a.b.c.d.e")`},
		{expr: `hierarchy("a:b:c:d", ":") == hierarchy("a.b.c.d")`},
		{expr: `hierarchy("aFOObFOOcFOOd", "FOO") == hierarchy("a.b.c.d")`},
		{expr: `hierarchy(["a","b","c","d"]) == hierarchy("a.b.c.d")`},
		{expr: `hierarchy("a.b.c.d").size() == 4`},
		{expr: `hierarchy("a.b.c.d")[2] == "c"`},
		{expr: `hierarchy("a.b").ancestorOf(hierarchy("a.b.c.d.e"))`},
		{expr: `hierarchy("a.b").ancestorOf(hierarchy("a.b")) == false`},
		{expr: `hierarchy("a.b.c.d").ancestorOf(hierarchy("a.b.c.d.e"))`},
		{expr: `hierarchy("x.y.c.d").ancestorOf(hierarchy("a.b.c.d.e")) == false`},
		{expr: `hierarchy("a.b.c.d").commonAncestors(hierarchy("a.b.c.d.e")) == hierarchy("a.b.c.d")`},
		{expr: `hierarchy("a.b.c.d").commonAncestors(hierarchy("a.b.c.d")) == hierarchy("a.b.c")`},
		{expr: `hierarchy("a.b.c.d").commonAncestors(hierarchy("x.y.z")).size() == 0`},
		{expr: `hierarchy("a.b.c.d.e").descendentOf(hierarchy("a.b"))`},
		{expr: `hierarchy("a.b").descendentOf(hierarchy("a.b")) == false`},
		{expr: `hierarchy("x.b").descendentOf(hierarchy("a.b")) == false`},
		{expr: `hierarchy("a.b.c.d.e").immediateChildOf(hierarchy("a.b.c.d"))`},
		{expr: `hierarchy("a.b.c.d.e").immediateChildOf(hierarchy("a.b.c")) == false`},
		{expr: `hierarchy("a.b.c.d").immediateParentOf(hierarchy("a.b.c.d.e"))`},
		{expr: `!hierarchy("a.b.c").immediateParentOf(hierarchy("a.b.c.d.e"))`},
		{expr: `hierarchy("a.b.c").siblingOf(hierarchy("a.b.d"))`},
		{expr: `hierarchy("a.b.c").siblingOf(hierarchy("x.b.d")) == false`},
		{expr: `hierarchy("a.b.c.d").siblingOf(hierarchy("a.b.d")) == false`},
		{expr: `hierarchy("a.b.c.d").overlaps(hierarchy("a.b.c.d")) == true`},
		{expr: `hierarchy("a.b.c.d").overlaps(hierarchy("a.b.c")) == true`},
		{expr: `hierarchy("a.b").overlaps(hierarchy("a.b.c.d")) == true`},
		{expr: `hierarchy("a.b.x").overlaps(hierarchy("a.b.c.d")) == false`},
		{expr: `ldapDN("cn=admin,ou=users,dc=example,dc=com") == ldapDN("cn=admin,ou=users,dc=example,dc=com")`},
		{expr: `ldapDN("cn=admin,ou=users,dc=example,dc=com") != ldapDN("cn=admin,ou=groups,dc=example,dc=com")`},
		{expr: `ldapDN("dc=example,dc=com").ancestorOf(ldapDN("cn=admin,ou=users,dc=example,dc=com"))`},
		{expr: `ldapDN("dc=example,dc=com").ancestorOf(ldapDN("dc=example,dc=com")) == false`},
		{expr: `ldapDN("ou=users,dc=example,dc=com").ancestorOf(ldapDN("cn=admin,ou=users,dc=example,dc=com"))`},
		{expr: `ldapDN("cn=admin,ou=users,dc=example,dc=com").ancestorOf(ldapDN("dc=example,dc=com")) == false`},
		{expr: `ldapDN("dc=example,dc=com").isOrAncestorOf(ldapDN("cn=admin,ou=users,dc=example,dc=com"))`},
		{expr: `ldapDN("dc=example,dc=com").isOrAncestorOf(ldapDN("DC=Example,dc=com"))`},
		{expr: `ldapDN("cn=admin,ou=users,dc=example,dc=com").isOrAncestorOf(ldapDN("ou=users,dc=example,dc=com")) == false`},
		{expr: `ldapDN("ou=groups,dc=example,dc=com").isOrAncestorOf(ldapDN("cn=admin,ou=users,dc=example,dc=com")) == false`},
		{expr: `ldapDN("ou=users,dc=example,dc=com").scopeBase(ldapDN("OU=Users,dc=example,dc=com"))`},
		{expr: `ldapDN("ou=users,dc=example,dc=com").scopeBase(ldapDN("cn=admin,ou=users,dc=example,dc=com")) == false`},
		{expr: `ldapDN("ou=users,dc=example,dc=com").scopeOneLevel(ldapDN("cn=admin,ou=users,dc=example,dc=com"))`},
		{expr: `ldapDN("ou=users,dc=example,dc=com").scopeOneLevel(ldapDN("ou=users,dc=example,dc=com")) == false`},
		{expr: `ldapDN("ou=users,dc=example,dc=com").scopeOneLevel(ldapDN("cn=x,cn=admin,ou=users,dc=example,dc=com")) == false`},
		{expr: `ldapDN("ou=users,dc=example,dc=com").scopeChildren(ldapDN("cn=x,cn=admin,ou=users,dc=example,dc=com"))`},
		{expr: `ldapDN("ou=users,dc=example,dc=com").scopeChildren(ldapDN("ou=users,dc=example,dc=com")) == false`},
		{expr: `ldapDN("ou=users,dc=example,dc=com").scopeSubtree(ldapDN("ou=users,dc=example,dc=com"))`},
		{expr: `ldapDN("ou=users,dc=example,dc=com").scopeSubtree(ldapDN("cn=x,cn=admin,ou=users,dc=example,dc=com"))`},
		{expr: `ldapDN("ou=users,dc=example,dc=com").scopeSubtree(ldapDN("dc=example,dc=com")) == false`},
		{expr: `ldapDN("ou=users,dc=example,dc=com").scopeSubtree(ldapDN("cn=admin,ou=groups,dc=example,dc=com")) == false`},
		{expr: `ldapDN("cn=admin,ou=users,dc=example,dc=com").descendentOf(ldapDN("dc=example,dc=com"))`},
		{expr: `ldapDN("dc=example,dc=com").descendentOf(ldapDN("dc=example,dc=com")) == false`},
		{expr: `ldapDN("ou=users,dc=example,dc=com").immediateParentOf(ldapDN("cn=admin,ou=users,dc=example,dc=com"))`},
		{expr: `ldapDN("ou=users,dc=example,dc=com").immediateParentOf(ldapDN("cn=admin,ou=groups,ou=users,dc=example,dc=com")) == false`},
		{expr: `ldapDN("cn=admin,ou=users,dc=example,dc=com").immediateChildOf(ldapDN("ou=users,dc=example,dc=com"))`},
		{expr: `ldapDN("cn=admin,ou=users,dc=example,dc=com").immediateChildOf(ldapDN("dc=example,dc=com")) == false`},
		{expr: `ldapDN("cn=admin,ou=users,dc=example,dc=com").siblingOf(ldapDN("cn=admin,ou=groups,dc=example,dc=com")) == false`},
		{expr: `ldapDN("cn=admin,ou=users,dc=example,dc=com").siblingOf(ldapDN("cn=guest,ou=users,dc=example,dc=com"))`},
		{expr: `ldapDN("cn=admin,ou=users,dc=example,dc=com").siblingOf(ldapDN("cn=admin,ou=users,dc=example,dc=com")) == false`},
		{expr: `ldapDN("cn=admin,ou=users,dc=example,dc=com").siblingOf(ldapDN("cn=admin,dc=example,dc=com")) == false`},
		{expr: `ldapDN("ou=users,dc=example,dc=com").overlaps(ldapDN("cn=admin,ou=users,dc=example,dc=com"))`},
		{expr: `ldapDN("cn=admin,ou=users,dc=example,dc=com").overlaps(ldapDN("ou=users,dc=example,dc=com"))`},
		{expr: `ldapDN("ou=users,dc=example,dc=com").overlaps(ldapDN("ou=groups,dc=example,dc=com")) == false`},
		{expr: `ldapDN("cn=admin,ou=users,dc=example,dc=com").commonAncestors(ldapDN("cn=guest,ou=users,dc=example,dc=com")) == ldapDN("ou=users,dc=example,dc=com")`},
		{expr: `ldapDN("dc=example,dc=com").commonAncestors(ldapDN("ou=users,dc=example,dc=com")) == ldapDN("dc=example,dc=com")`},
		{expr: `ldapDN("cn=admin,ou=users,dc=example,dc=com").size() == 4`},
		{expr: `ldapDN("dc=com").size() == 1`},
		{expr: `ldapDN("cn=admin,ou=users,dc=example,dc=com")[0] == "cn=admin"`},
		{expr: `ldapDN("cn=admin,ou=users,dc=example,dc=com")[1] == "ou=users"`},
		{expr: `ldapDN("cn=admin,ou=users,dc=example,dc=com")[2] == "dc=example"`},
		{expr: `ldapDN("cn=admin+ou=users,dc=example,dc=com").size() == 3`},
		{expr: `ldapDN("CN=admin,OU=users,DC=example,DC=com") == ldapDN("cn=admin,ou=users,dc=example,dc=com")`},
		{expr: `ldapDN("ou=users,dc=example,dc=com") == ldapDN("  ou=users,dc=example,dc=com ")`},
		{expr: `ldapDN("CN=admin,OU=users,DC=example,DC=com") == ldapDN("cn=admin,ou=users,dc=example,dc=com")`},
		{expr: `ldapDN("CN = admin , OU = users , DC = example , DC=com") == ldapDN("cn=admin,ou=users,dc=example,dc=com")`},
		{expr: `ldapDN("cn=ad  min,ou=users,dc=example,dc=com") == ldapDN("cn=ad min,ou=users,dc=example,dc=com")`},
		{expr: `ldapDN("cn=ad\\2Bmin,dc=com") == ldapDN("cn=ad\\+min,dc=com")`},
		{expr: `ldapDN("cn=ad\\2Bmin,dc=com")[0] == "cn=ad\\+min"`},
		{expr: `ldapDN("cn=admin+ou=users,dc=com").size() == 2`},
		{expr: `ldapDN("cn=admin+ou=users,dc=com") == ldapDN("ou=users+cn=admin,dc=com")`},
		{expr: `ldapDN("cn=x+cn=x,dc=com") != ldapDN("cn=x+ou=y,dc=com")`},
		{expr: `ldapDN("cn=x+ou=y,dc=com") != ldapDN("cn=x+cn=x,dc=com")`},
		{expr: `ldapDN("cn=a+cn=b,dc=com") != ldapDN("cn=a\\+2.5.4.3=b,dc=com")`},
		{expr: `ldapDN("cn=a,cn=b,dc=com") != ldapDN("cn=a\\,2.5.4.3=b,dc=com")`},
		{expr: `ldapDNList(["cn=a+cn=b,dc=com"]).hasDN("cn=a\\+2.5.4.3=b,dc=com") == false`},
		{expr: `ldapDNList(["cn=a,cn=b,dc=com"]).anyIn(["cn=a\\,2.5.4.3=b,dc=com"]) == false`},
		{expr: `ldapDN("cn=foo") == ldapDN("commonName=foo")`},
		{expr: `ldapDN("cn=foo") == ldapDN("2.5.4.3=foo")`},
		{expr: `ldapDN("cn=Domain Admins") == ldapDN("cn=Domain      Admins")`},
		// Case-insensitive attribute values
		{expr: `ldapDN("cn=admin,ou=users,dc=example,dc=com") == ldapDN("CN=ADMIN,OU=USERS,dc=example,dc=com")`},
		{expr: `ldapDN("cn=admin,ou=users,dc=example,dc=com").ancestorOf(ldapDN("CN=ADMIN,OU=USERS,dc=example,dc=com")) == false`},
		{expr: `ldapDN("ou=users,dc=example,dc=com").ancestorOf(ldapDN("CN=admin,ou=Users,dc=example,dc=com"))`},
		{expr: `ldapDN("cn=admin,dc=EXAMPLE,dc=com") == ldapDN("cn=admin,dc=example,dc=com")`},
		{expr: `ldapDN("uid=Alice,dc=example,dc=com") == ldapDN("UID=alice,dc=example,dc=com")`},
		{expr: `ldapDN("uid=alice,cn=users,dc=Example,dc=com").descendentOf(ldapDN("dc=example,dc=com"))`},
		{expr: `ldapDN("univentionAppID=Foo,dc=com") != ldapDN("UNIVENTIONAPPID=foo,dc=com")`},
		{expr: `ldapDN("univentionAppID=Foo,dc=com") == ldapDN("UNIVENTIONAPPID=Foo,DC=COM")`},
		{expr: `ldapDN("univentionAppID=Foo,dc=com") == ldapDN("1.3.6.1.4.1.10176.4204.1.1=Foo,dc=com")`},
		{expr: `ldapDN("univentionAppID=Foo,dc=com") != ldapDN("1.3.6.1.4.1.10176.4204.1.1=foo,dc=com")`},
		{expr: `ldapDN("krb5PrincipalName=host/ucs.example.com@EXAMPLE.COM,cn=kerberos,dc=com") == ldapDN("KRB5PRINCIPALNAME=host/ucs.example.com@EXAMPLE.COM,CN=Kerberos,dc=com")`},
		{expr: `ldapDN("krb5PrincipalName=host/ucs.example.com@EXAMPLE.COM,cn=kerberos,dc=com") == ldapDN("1.3.6.1.4.1.5322.10.1.1=host/ucs.example.com@EXAMPLE.COM,cn=kerberos,dc=com")`},
		{expr: `ldapDN("krb5PrincipalName=host/ucs.example.com@EXAMPLE.COM,cn=kerberos,dc=com") != ldapDN("krb5PrincipalName=host/ucs.example.com@example.com,cn=kerberos,dc=com")`},
		{expr: `ldapDN("krb5PrincipalName=a  b,dc=com") != ldapDN("krb5PrincipalName=a b,dc=com")`},
		{expr: `ldapDN("krb5PrincipalName=a\u00ADb,dc=com") != ldapDN("krb5PrincipalName=ab,dc=com")`},
		{expr: `ldapDN("univentionObjectIdentifier=0f8b2c4e-6a1d-4e3f-9b7a-2c5d8e1f4a6b,dc=com") == ldapDN("1.3.6.1.4.1.10176.1003.3=0F8B2C4E-6A1D-4E3F-9B7A-2C5D8E1F4A6B,dc=com")`},
		{expr: `ldapDN("cn=admin,dc=com") == ldapDN("commonName=admin,domainComponent=com")`},
		// RFC 4518 string preparation
		{expr: `ldapDN("cn=Straße,dc=com") == ldapDN("cn=STRASSE,dc=com")`},
		{expr: `ldapDN("cn=\uFF41dmin,dc=com") == ldapDN("cn=admin,dc=com")`},
		{expr: `ldapDN("cn=ad\u00ADmin,dc=com") == ldapDN("cn=admin,dc=com")`},
		{expr: `ldapDN("cn=Domain\tAdmins,dc=com") == ldapDN("cn=domain admins,dc=com")`},
		{expr: `ldapDN("cn=\uE000,dc=com") == ldapDN("cn=x,dc=com")`, wantErr: true},
		{expr: `ldapDNList(["cn=\uE000,dc=com"]).size() == 1`, wantErr: true},
		{expr: `ldapDN("ou=users,dc=example,dc=com").immediateParentOf(ldapDN("cn=Admin,ou=Users,dc=example,dc=com"))`},
		{expr: `ldapDN("cn=Admin,ou=Users,dc=example,dc=com").immediateChildOf(ldapDN("ou=users,dc=example,dc=com"))`},
		{expr: `ldapDN("cn=Admin,ou=users,dc=example,dc=com").siblingOf(ldapDN("cn=Guest,ou=users,dc=example,dc=com"))`},
		{expr: `ldapDN("cn=Admin,ou=users,dc=example,dc=com").commonAncestors(ldapDN("cn=Guest,ou=Users,dc=example,dc=com")) == ldapDN("ou=users,dc=example,dc=com")`},
		// ldapDNList constructor and basic access
		{expr: `ldapDNList(["cn=admin,ou=users,dc=example,dc=com"]).size() == 1`},
		{expr: `ldapDNList([]).size() == 0`},
		// hasDN
		{expr: `ldapDNList(["cn=admin,ou=users,dc=example,dc=com"]).hasDN("cn=admin,ou=users,dc=example,dc=com")`},
		{expr: `ldapDNList(["cn=admin,ou=users,dc=example,dc=com"]).hasDN("CN=Admin,OU=Users,dc=example,dc=com")`},
		{expr: `ldapDNList(["cn=admin,ou=users,dc=example,dc=com"]).hasDN("cn=guest,ou=users,dc=example,dc=com") == false`},
		{expr: `ldapDNList([]).hasDN("cn=admin,dc=com") == false`},
		// allIn
		{expr: `ldapDNList(["cn=a,dc=com","cn=b,dc=com"]).allIn(["cn=a,dc=com","cn=b,dc=com","cn=c,dc=com"])`},
		{expr: `ldapDNList(["cn=a,dc=com","cn=x,dc=com"]).allIn(["cn=a,dc=com","cn=b,dc=com"]) == false`},
		{expr: `ldapDNList([]).allIn(["cn=a,dc=com"])`},
		// anyIn
		{expr: `ldapDNList(["cn=a,dc=com","cn=b,dc=com"]).anyIn(["cn=b,dc=com","cn=c,dc=com"])`},
		{expr: `ldapDNList(["cn=a,dc=com"]).anyIn(["cn=b,dc=com"]) == false`},
		{expr: `ldapDNList([]).anyIn(["cn=a,dc=com"]) == false`},
		// subsetOf
		{expr: `ldapDNList(["cn=a,dc=com"]).subsetOf(["cn=a,dc=com","cn=b,dc=com"])`},
		{expr: `ldapDNList(["cn=a,dc=com","cn=x,dc=com"]).subsetOf(["cn=a,dc=com","cn=b,dc=com"]) == false`},
		{expr: `ldapDNList([]).subsetOf(["cn=a,dc=com"])`},
		// intersects
		{expr: `ldapDNList(["cn=a,dc=com","cn=b,dc=com"]).intersects(["cn=b,dc=com","cn=c,dc=com"])`},
		{expr: `ldapDNList(["cn=a,dc=com"]).intersects(["cn=b,dc=com"]) == false`},
		{expr: `ldapDNList([]).intersects(["cn=a,dc=com"]) == false`},
		// equalsDN (same set, order-independent)
		{expr: `ldapDNList(["cn=a,dc=com","cn=b,dc=com"]).equalsDN(["cn=b,dc=com","cn=a,dc=com"])`},
		{expr: `ldapDNList(["cn=a,dc=com"]).equalsDN(["cn=a,dc=com","cn=b,dc=com"]) == false`},
		{expr: `ldapDNList([]).equalsDN([])`},
		{expr: `ldapDNList(["cn=a,dc=com","cn=a,dc=com"]).equalsDN(["cn=a,dc=com","cn=b,dc=com"]) == false`},
		{expr: `ldapDNList(["cn=a,dc=com","cn=b,dc=com"]).equalsDN(["cn=a,dc=com","cn=a,dc=com"]) == false`},
		{expr: `ldapDNList(["cn=a,dc=com","cn=a,dc=com"]).equalsDN(["cn=a,dc=com"])`},
		{expr: `ldapDNList(["cn=Admin,dc=com","cn=admin,dc=com"]).equalsDN(["cn=admin,dc=com","cn=b,dc=com"]) == false`},
		// == on ldapDNList has the same set semantics as equalsDN
		{expr: `ldapDNList(["cn=a,dc=com","cn=b,dc=com"]) == ldapDNList(["cn=b,dc=com","cn=a,dc=com"])`},
		{expr: `ldapDNList(["cn=a,dc=com","cn=a,dc=com"]) == ldapDNList(["cn=a,dc=com"])`},
		{expr: `ldapDNList(["cn=a,dc=com","cn=a,dc=com"]) != ldapDNList(["cn=a,dc=com","cn=b,dc=com"])`},
		// case-insensitive equality preserved through set ops
		{expr: `ldapDNList(["CN=Admin,OU=Users,dc=example,dc=com"]).subsetOf(["cn=admin,ou=users,dc=example,dc=com"])`},
		// anyDescendentOf
		{expr: `ldapDNList(["cn=admin,ou=users,dc=example,dc=com","cn=guest,ou=other,dc=example,dc=com"]).anyDescendentOf("ou=users,dc=example,dc=com")`},
		{expr: `ldapDNList(["cn=admin,ou=users,dc=example,dc=com"]).anyDescendentOf("ou=groups,dc=example,dc=com") == false`},
		{expr: `ldapDNList([]).anyDescendentOf("dc=example,dc=com") == false`},
		// allDescendentOf
		{expr: `ldapDNList(["cn=admin,ou=users,dc=example,dc=com","cn=guest,ou=users,dc=example,dc=com"]).allDescendentOf("ou=users,dc=example,dc=com")`},
		{expr: `ldapDNList(["cn=admin,ou=users,dc=example,dc=com","cn=guest,ou=other,dc=example,dc=com"]).allDescendentOf("ou=users,dc=example,dc=com") == false`},
		{expr: `ldapDNList([]).allDescendentOf("dc=example,dc=com")`},
		// case-insensitive base DN for hierarchy methods
		{expr: `ldapDNList(["cn=admin,ou=users,dc=example,dc=com"]).anyDescendentOf("OU=Users,dc=example,dc=com")`},
		{expr: `now().timeSince() == duration("0")`},
		{expr: `now() == now()`},
		{expr: `spiffeID("spiffe://cerbos.dev/ns/privileged/sa/curl").trustDomain() == spiffeTrustDomain("cerbos.dev")`},
		{expr: `spiffeID("spiffe://cerbos.dev/ns/privileged/sa/curl").path() == "/ns/privileged/sa/curl"`},
		{expr: `spiffeID("spiffe://cerbos.dev/ns/privileged/sa/curl").isMemberOf(spiffeTrustDomain("cerbos.dev"))`},
		{expr: `spiffeID("spiffe://cerbos.dev/ns/privileged/sa/curl").isMemberOf(spiffeTrustDomain("spiffe://cerbos.dev"))`},
		{expr: `spiffeTrustDomain("spiffe://cerbos.dev").name() == "cerbos.dev"`},
		{expr: `spiffeTrustDomain("spiffe://cerbos.dev").id() == "spiffe://cerbos.dev"`},
		{expr: `spiffeTrustDomain("cerbos.dev").id() == "spiffe://cerbos.dev"`},
		{expr: `spiffeTrustDomain("spiffe://cerbos.dev/ns/privileged/sa/curl") == spiffeTrustDomain("spiffe://cerbos.dev")`},
		{expr: `spiffeMatchAny().matchesID("spiffe://cerbos.dev/ns/privileged/sa/curl")`},
		{expr: `spiffeMatchAny().matchesID(spiffeID("spiffe://cerbos.dev/ns/privileged/sa/curl"))`},
		{expr: `spiffeMatchExact(spiffeID("spiffe://cerbos.dev/ns/privileged/sa/curl")).matchesID(spiffeID("spiffe://cerbos.dev/ns/privileged/sa/curl"))`},
		{expr: `!spiffeMatchExact(spiffeID("spiffe://cerbos.dev/ns/privileged/sa/curl")).matchesID(spiffeID("spiffe://cerbos.dev/ns/privileged/sa/foo"))`},
		{expr: `spiffeMatchOneOf([spiffeID("spiffe://cerbos.dev/ns/privileged/sa/curl"), spiffeID("spiffe://cerbos.dev/ns/unprivileged/sa/curl")]).matchesID(spiffeID("spiffe://cerbos.dev/ns/privileged/sa/curl"))`},
		{expr: `!spiffeMatchOneOf([spiffeID("spiffe://cerbos.dev/ns/privileged/sa/curl"), spiffeID("spiffe://cerbos.dev/ns/unprivileged/sa/curl")]).matchesID(spiffeID("spiffe://cerbos.dev/ns/privileged/sa/foo"))`},
		{expr: `spiffeMatchOneOf(["spiffe://cerbos.dev/ns/privileged/sa/curl", "spiffe://cerbos.dev/ns/unprivileged/sa/curl"]).matchesID(spiffeID("spiffe://cerbos.dev/ns/privileged/sa/curl"))`},
		{expr: `spiffeMatchTrustDomain(spiffeTrustDomain("spiffe://cerbos.dev")).matchesID(spiffeID("spiffe://cerbos.dev/ns/privileged/sa/curl"))`},
		{expr: `spiffeMatchTrustDomain("spiffe://cerbos.dev").matchesID(spiffeID("spiffe://cerbos.dev/ns/privileged/sa/curl"))`},
		{expr: `!spiffeMatchTrustDomain("spiffe://cerbos.dev").matchesID(spiffeID("spiffe://example.com/ns/privileged/sa/curl"))`},
		{expr: `basePath("/path/to/dir") == "dir"`},
		{expr: `dirPath("/path/to/file.txt") == "/path/to"`},
		{expr: `extPath("/path/to/file.txt") == ".txt"`},
		{expr: `joinPath(["/path", "to", "file.txt"]) == "/path/to/file.txt"`},
		{expr: `joinPath([]) == ""`},
		{expr: `pathHasPrefix("/path/to/dir", "/path/to") == true`},
		{expr: `pathHasPrefix("\\\\host\\share\\dir", "\\\\host\\share") == true`},
		{expr: `pathHasPrefix("C:\\path\\to\\dir", "C:\\path\\to") == true`},
		{expr: `pathHasPrefix("/foo/barbaz", "/foo/bar") == false`},
		{expr: `pathHasPrefix("\\\\host\\share\\barbaz", "\\\\host\\share\\bar") == false`},
		{expr: `pathHasPrefix("C:\\foo\\barbaz", "C:\\foo\\bar") == false`},
		{expr: `pathHasPrefix("/path/to/dir", "/path/to/dir") == true`},
		{expr: `pathHasPrefix("\\\\host\\share\\dir", "\\\\host\\share\\dir") == true`},
		{expr: `pathHasPrefix("C:\\path\\to\\dir", "C:\\path\\to\\dir") == true`},
		{expr: `"/path/to/dir".pathHasPrefix("/path/to") == true`},
		{expr: `"\\\\host\\share\\dir".pathHasPrefix("\\\\host\\share") == true`},
		{expr: `"C:\\path\\to\\dir".pathHasPrefix("C:\\path\\to") == true`},
		{expr: `"/foo/barbaz".pathHasPrefix("/foo/bar") == false`},
		{expr: `"\\\\host\\share\\barbaz".pathHasPrefix("\\\\host\\share\\bar") == false`},
		{expr: `"C:\\foo\\barbaz".pathHasPrefix("C:\\foo\\bar") == false`},
		{expr: `"/path/to/dir".pathHasPrefix("/path/to/dir") == true`},
		{expr: `"\\\\host\\share\\dir".pathHasPrefix("\\\\host\\share\\dir") == true`},
		{expr: `"C:\\path\\to\\dir".pathHasPrefix("C:\\path\\to\\dir") == true`},
		{expr: `pathMatch("/path/to/file.zip", "/path/to/*.zip") == true`},
		{expr: `pathMatch("/path/to/file.zip", "/path/to/*.txt") == false`},
		{expr: `"/path/to/file.zip".pathMatch("/path/to/*.zip") == true`},
		{expr: `"/path/to/file.zip".pathMatch("/path/to/*.txt") == false`},
		{expr: `pathMatchAnyOf("/path/to/file.zip", ["/path/to/*.tar.gz", "/path/to/*.zip"]) == true`},
		{expr: `pathMatchAnyOf("\\\\host\\share\\file.zip", ["\\\\host\\share\\*.tar.gz", "\\\\host\\share\\*.zip"]) == true`},
		{expr: `pathMatchAnyOf("C:\\path\\to\\file.zip", ["C:\\path\\to\\*.tar.gz", "C:\\path\\to\\*.zip"]) == true`},
		{expr: `"/path/to/file.zip".pathMatchAnyOf(["/path/to/*.tar.gz", "/path/to/*.zip"]) == true`},
		{expr: `"\\\\host\\share\\file.zip".pathMatchAnyOf(["\\\\host\\share\\*.tar.gz", "\\\\host\\share\\*.zip"]) == true`},
		{expr: `"C:\\path\\to\\file.zip".pathMatchAnyOf(["C:\\path\\to\\*.tar.gz", "C:\\path\\to\\*.zip"]) == true`},
		{expr: `relPath("/path/to/dir", "/path/to/other_dir") == "../other_dir"`},
		{expr: `volumeName("/path/to/dir") == ""`},
	}
	env, err := cel.NewEnv(conditions.CerbosCELLib())
	require.NoError(t, err)

	for _, tc := range testCases {
		t.Run(tc.expr, func(t *testing.T) {
			is := require.New(t)
			ast, issues := env.Compile(tc.expr)
			is.NoError(issues.Err())

			have, _, err := conditions.ContextEval(t.Context(), env, ast.NativeRep(), cel.NoVars(), conditions.Now(), cel.EvalOptions(cel.OptOptimize))
			if tc.wantErr {
				is.Error(err)
			} else {
				is.NoError(err)
				is.Equal(true, have.Value())
			}
		})
	}
}

func TestCmpSelectAndCall(t *testing.T) {
	t.Skip()
	env, _ := cel.NewEnv(
		cel.Types(&enginev1.Resource{}),
		cel.VariableDecls(
			decls.NewVariable("y", celtypes.NewListType(celtypes.StringType)),
			decls.NewVariable(conditions.CELResourceAbbrev, types.MessageType[*enginev1.Resource]()),
			decls.NewVariable("z", celtypes.StringType),
			decls.NewVariable("v", celtypes.NewMapType(celtypes.StringType, celtypes.DynType)),
		),
		ext.Strings(),
	)

	expr0 := "v.geo() in (y + [z]).map(t, t.upperAscii())"
	expr := "v.geo in (y + [z]).map(t, t.upperAscii())"

	ast, issues := env.Parse(expr)
	is := require.New(t)
	if issues != nil {
		is.NoError(issues.Err())
	}
	ast0, issues := env.Parse(expr0)
	if issues != nil {
		is.NoError(issues.Err())
	}

	v := reflect.DeepEqual(*ast0, *ast)
	is.True(v)
}

func TestPartialEvaluationWithMacroGlobalVars(t *testing.T) {
	expander := func(eh parser.ExprHelper, _ celast.Expr, _ []celast.Expr) (celast.Expr, *common.Error) {
		sel := eh.NewSelect(eh.NewSelect(eh.NewIdent("R"), "attr"), "geo")
		return sel, nil
	}
	geo := parser.NewReceiverMacro("geo", 0, expander)
	env, _ := cel.NewEnv(
		cel.Types(&enginev1.Resource{}),
		cel.VariableDecls(
			decls.NewVariable("y", celtypes.NewListType(celtypes.StringType)),
			decls.NewVariable(conditions.CELResourceAbbrev, types.MessageType[*enginev1.Resource]()),
			decls.NewVariable("z", celtypes.StringType),
		),
		// decls.NewVar("v", decls.NewMapType(decls.String, decls.Dyn))),
		ext.Strings(),
		cel.Macros(geo),
	)

	vars, _ := cel.PartialVars(map[string]any{
		"y": []string{"GB", "US"},
		"z": "ca",
		"v": map[string]any{},
	}, cel.AttributePattern("R"))

	expr := "v.geo() in (y + [z]).map(t, t.upperAscii())"
	want := `R.attr.geo in ["GB", "US", "CA"]`

	is := require.New(t)
	ast, issues := env.Compile(expr)
	if issues != nil {
		is.NoError(issues.Err())
	}

	out, det, err := conditions.ContextEval(t.Context(), env, ast.NativeRep(), vars, time.Now, cel.EvalOptions(cel.OptTrackState, cel.OptPartialEval))
	is.NoError(err)
	is.True(celtypes.IsUnknown(out))
	residual, err := env.ResidualAst(ast, det)
	is.NoError(err)
	astToString, err := cel.AstToString(residual)
	is.NoError(err)
	is.Equal(want, astToString)
}

func TestPartialEvaluation(t *testing.T) {
	env, _ := cel.NewEnv(
		cel.Types(&enginev1.Resource{}),
		cel.VariableDecls(
			decls.NewVariable(conditions.CELRequestIdent, types.MessageType[*enginev1.CheckInput]()),
			decls.NewVariable("y", celtypes.NewListType(celtypes.StringType)),
			decls.NewVariable(conditions.CELResourceAbbrev, types.MessageType[*enginev1.Resource]()),
			decls.NewVariable("request.principal", celtypes.NewMapType(celtypes.StringType, celtypes.DynType)),
			decls.NewVariable("z", celtypes.StringType),
			decls.NewVariable("R.attr.department", celtypes.StringType),
		),
		ext.Strings(),
	)

	vars, _ := cel.PartialVars(map[string]any{
		"y":                 []string{"GB", "US"},
		"z":                 "ca",
		"R.attr.department": "marketing",
		"request.principal": map[string]any{
			"attr": map[string]any{
				"country": "NZ",
			},
		},
	},
		cel.AttributePattern("request").QualString("resource"),
		cel.AttributePattern("R"),
	)

	tests := []struct {
		expr, result string
	}{
		{
			expr:   `R.attr.department == R.attr.project.department`,
			result: `"marketing" == R.attr.project.department`,
		},
		{
			expr:   `R.attr.geo in y && R.attr.department in ["engineering", "design"]`,
			result: `false`,
		},
		{
			expr:   "R.attr.geo in (y + [z]).map(t, t.upperAscii())",
			result: `R.attr.geo in ["GB", "US", "CA"]`,
		},
		{
			expr:   "request.resource.attr.geo == request.principal.attr.country",
			result: `request.resource.attr.geo == "NZ"`,
		},
		{
			expr:   "R.attr.geo in (y + [z]).map(t, t.upperAscii()) || 1 == 1",
			result: "true",
		},
		{
			expr:   `"CA" in (y + [z]).map(t, t.upperAscii())`,
			result: "true",
		},
		{
			expr:   `("CA" in (y + [z]).map(t, t.upperAscii())) && 1 == 2`,
			result: "false",
		},
		{
			expr:   `("NZ" in (y + [z]).map(t, t.upperAscii()))`,
			result: "false",
		},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			is := require.New(t)
			expr := tt.expr
			ast, issues := env.Compile(expr)
			if issues != nil {
				is.NoError(issues.Err())
			}

			out, det, err := conditions.ContextEval(t.Context(), env, ast.NativeRep(), vars, time.Now, cel.EvalOptions(cel.OptTrackState, cel.OptPartialEval))
			is.NotNil(det) // It is not nil if cel.OptTrackState is included in the cel.EvalOptions
			t.Log(out.Type())
			is.NoError(err)
			residual, err := env.ResidualAst(ast, det)
			is.NoError(err)
			astToString, err := cel.AstToString(residual)
			is.NoError(err)
			is.Equal(tt.result, astToString)
			log.Print(astToString)
		})
	}
}
