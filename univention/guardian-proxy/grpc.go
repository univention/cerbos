// SPDX-FileCopyrightText: 2026 Univention GmbH
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"context"

	svcv1 "github.com/cerbos/cerbos/api/genpb/cerbos/svc/v1"
	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// newGRPCServer serves the Cerbos API and health over gRPC. The admin API and
// AuthZEN are not registered, so clients get Unimplemented for them.
func newGRPCServer(p *proxy, health healthpb.HealthClient) *grpc.Server {
	s := grpc.NewServer()
	svcv1.RegisterCerbosServiceServer(s, p)
	healthpb.RegisterHealthServer(s, healthProxy{upstream: health})
	return s
}

// healthProxy reports the health of Cerbos, as /_cerbos/health does over HTTP.
type healthProxy struct {
	healthpb.UnimplementedHealthServer
	upstream healthpb.HealthClient
}

func (h healthProxy) Check(ctx context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	return h.upstream.Check(ctx, req)
}
