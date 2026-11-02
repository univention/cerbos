// SPDX-FileCopyrightText: 2026 Univention GmbH
// SPDX-License-Identifier: AGPL-3.0-only

package guardianroles

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	enginev1 "github.com/cerbos/cerbos/api/genpb/cerbos/engine/v1"
	requestv1 "github.com/cerbos/cerbos/api/genpb/cerbos/request/v1"
)

// MapPrincipal is the hook for the role mapping (guardian#302).
// It may change the principal in place; an error rejects the request.
func MapPrincipal(_ context.Context, _ *enginev1.Principal) error {
	return nil
}

// UnaryServerInterceptor calls MapPrincipal for every decision and playground request.
// Cerbos serves HTTP through its gRPC server, so this covers both protocols.
// It must run before request validation, so that the mapped principal is validated.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return interceptor(MapPrincipal)
}

func principalOf(req any) *enginev1.Principal {
	switch r := req.(type) {
	case *requestv1.CheckResourcesRequest:
		return r.GetPrincipal()
	case *requestv1.CheckResourceSetRequest:
		return r.GetPrincipal()
	case *requestv1.CheckResourceBatchRequest:
		return r.GetPrincipal()
	case *requestv1.PlanResourcesRequest:
		return r.GetPrincipal()
	case *requestv1.PlaygroundEvaluateRequest:
		return r.GetPrincipal()
	case *requestv1.PlaygroundProxyRequest:
		switch p := r.GetProxyRequest().(type) {
		case *requestv1.PlaygroundProxyRequest_CheckResources:
			return principalOf(p.CheckResources)
		case *requestv1.PlaygroundProxyRequest_CheckResourceSet:
			return principalOf(p.CheckResourceSet)
		case *requestv1.PlaygroundProxyRequest_CheckResourceBatch:
			return principalOf(p.CheckResourceBatch)
		case *requestv1.PlaygroundProxyRequest_PlanResources:
			return principalOf(p.PlanResources)
		}
	}
	return nil
}

func interceptor(mapPrincipal func(context.Context, *enginev1.Principal) error) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		// A request without a principal fails validation after this interceptor.
		if principal := principalOf(req); principal != nil {
			if err := mapPrincipal(ctx, principal); err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "mapping principal: %v", err)
			}
		}

		return handler(ctx, req)
	}
}
