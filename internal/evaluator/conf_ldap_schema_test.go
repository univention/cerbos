// SPDX-FileCopyrightText: 2026 Zenauth Ltd.
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfValidateLDAPSchema(t *testing.T) {
	conf := &Conf{}
	conf.SetDefaults()
	require.NoError(t, conf.Validate())

	conf.Globals = map[string]any{"ldapSchema": map[string]any{
		"attributeTypes": []any{map[string]any{"oid": "2.5.4.3", "names": []any{"cn"}, "match": "ignoreCase"}},
	}}
	require.NoError(t, conf.Validate())

	conf.Globals = map[string]any{"ldapSchema": map[string]any{
		"attributeTypes": []any{map[string]any{"oid": "2.5.4.3", "names": []any{"cn"}, "match": "caseIgnoreMatch"}},
	}}
	require.ErrorContains(t, conf.Validate(), "engine.globals.ldapSchema: attributeTypes[0]: match: invalid value caseIgnoreMatch")
}
