// SPDX-FileCopyrightText: 2026 Univention GmbH
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"

	enginev1 "github.com/cerbos/cerbos/api/genpb/cerbos/engine/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

const guardianRolesAttr = "guardianRoles"

// PrincipalMapper changes the principal of a request before Cerbos evaluates
// it. If it returns an error, the request fails and is not forwarded.
type PrincipalMapper interface {
	MapPrincipal(ctx context.Context, p *enginev1.Principal) error
}

// staticMapper is a placeholder until guardian#302 defines the mapping. It adds
// the role mapped to each known guardianRoles value and keeps guardianRoles in
// the attributes, so that the audit log shows both.
type staticMapper struct {
	roles map[string]string
}

func loadStaticMapper(path string) (staticMapper, error) {
	m := staticMapper{roles: map[string]string{}}
	if path == "" {
		return m, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return m, err
	}

	if err := json.Unmarshal(data, &m.roles); err != nil {
		return m, fmt.Errorf("parsing %s: %w", path, err)
	}

	return m, nil
}

func (m staticMapper) MapPrincipal(_ context.Context, p *enginev1.Principal) error {
	v, ok := p.GetAttr()[guardianRolesAttr]
	if !ok {
		return nil
	}

	list := v.GetListValue()
	if list == nil {
		return fmt.Errorf("attr.%s must be a list of strings", guardianRolesAttr)
	}

	for _, elem := range list.GetValues() {
		if _, ok := elem.GetKind().(*structpb.Value_StringValue); !ok {
			return fmt.Errorf("attr.%s must be a list of strings", guardianRolesAttr)
		}

		role, ok := m.roles[elem.GetStringValue()]
		if ok && !slices.Contains(p.Roles, role) {
			p.Roles = append(p.Roles, role)
		}
	}

	return nil
}
