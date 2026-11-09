// SPDX-FileCopyrightText: 2026 Univention GmbH
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"context"
	"net"
	"net/http"
	"net/http/httputil"
	"net/textproto"
	"net/url"

	enginev1 "github.com/cerbos/cerbos/api/genpb/cerbos/engine/v1"
	requestv1 "github.com/cerbos/cerbos/api/genpb/cerbos/request/v1"
	responsev1 "github.com/cerbos/cerbos/api/genpb/cerbos/response/v1"
	svcv1 "github.com/cerbos/cerbos/api/genpb/cerbos/svc/v1"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

// proxy implements the Cerbos API by mapping the principal and forwarding the
// request to Cerbos over gRPC.
type proxy struct {
	svcv1.UnimplementedCerbosServiceServer
	upstream svcv1.CerbosServiceClient
	mapper   PrincipalMapper
}

func (p *proxy) CheckResourceSet(ctx context.Context, req *requestv1.CheckResourceSetRequest) (*responsev1.CheckResourceSetResponse, error) {
	return forward(ctx, p, req, p.upstream.CheckResourceSet)
}

func (p *proxy) CheckResourceBatch(ctx context.Context, req *requestv1.CheckResourceBatchRequest) (*responsev1.CheckResourceBatchResponse, error) {
	return forward(ctx, p, req, p.upstream.CheckResourceBatch)
}

func (p *proxy) CheckResources(ctx context.Context, req *requestv1.CheckResourcesRequest) (*responsev1.CheckResourcesResponse, error) {
	return forward(ctx, p, req, p.upstream.CheckResources)
}

func (p *proxy) PlanResources(ctx context.Context, req *requestv1.PlanResourcesRequest) (*responsev1.PlanResourcesResponse, error) {
	return forward(ctx, p, req, p.upstream.PlanResources)
}

func (p *proxy) ServerInfo(ctx context.Context, req *requestv1.ServerInfoRequest) (*responsev1.ServerInfoResponse, error) {
	return call(ctx, req, p.upstream.ServerInfo)
}

type principalRequest interface {
	GetPrincipal() *enginev1.Principal
}

func forward[Req principalRequest, Resp any](ctx context.Context, p *proxy, req Req, upstream func(context.Context, Req, ...grpc.CallOption) (Resp, error)) (Resp, error) {
	// A request without a principal goes to Cerbos unchanged, which rejects it.
	if principal := req.GetPrincipal(); principal != nil {
		if err := p.mapper.MapPrincipal(ctx, principal); err != nil {
			var zero Resp
			if _, ok := status.FromError(err); ok {
				return zero, err
			}
			return zero, status.Errorf(codes.InvalidArgument, "mapping principal: %v", err)
		}
	}

	return call(ctx, req, upstream)
}

// call passes the request metadata to Cerbos and the response headers back, so
// that the audit log and the client see what they would see without the proxy.
func call[Req, Resp any](ctx context.Context, req Req, upstream func(context.Context, Req, ...grpc.CallOption) (Resp, error)) (Resp, error) {
	var header metadata.MD
	resp, err := upstream(metadata.NewOutgoingContext(ctx, outgoingMetadata(ctx)), req, grpc.Header(&header))
	_ = grpc.SetHeader(ctx, header)
	return resp, err
}

// outgoingMetadata copies the incoming metadata. For gRPC clients it adds the
// client address to x-forwarded-for, as the gateway does for HTTP clients.
func outgoingMetadata(ctx context.Context) metadata.MD {
	in, _ := metadata.FromIncomingContext(ctx)
	md := in.Copy()
	if p, ok := peer.FromContext(ctx); ok {
		if host, _, err := net.SplitHostPort(p.Addr.String()); err == nil {
			md.Append("x-forwarded-for", host)
		}
	}

	return md
}

var decisionPaths = []string{
	"/api/check",
	"/api/check_resource_batch",
	"/api/check/resources",
	"/api/plan/resources",
	"/api/server_info",
}

// newHandler serves the Cerbos HTTP API the way Cerbos does, with grpc-gateway
// (internal/server/server.go). Paths other than the decision API are passed to
// the Cerbos HTTP API unchanged.
func newHandler(ctx context.Context, p *proxy, cerbosHTTP *url.URL) (http.Handler, error) {
	gw := runtime.NewServeMux(
		runtime.WithIncomingHeaderMatcher(incomingHeaderMatcher),
		runtime.WithMarshalerOption("application/json+pretty", &runtime.JSONPb{
			MarshalOptions: protojson.MarshalOptions{Indent: "  "},
		}),
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{}),
	)
	if err := svcv1.RegisterCerbosServiceHandlerServer(ctx, gw, p); err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	for _, path := range decisionPaths {
		mux.Handle(path, gw)
	}
	// The AuthZEN API takes a subject, which would bypass the mapping.
	mux.Handle("/access/v1/", http.NotFoundHandler())
	mux.Handle("/", httputil.NewSingleHostReverseProxy(cerbosHTTP))

	return mux, nil
}

// incomingHeaderMatcher is a copy of the one in internal/server/server.go, so
// that Cerbos receives the same metadata as from its own gateway.
func incomingHeaderMatcher(key string) (string, bool) {
	switch textproto.CanonicalMIMEHeaderKey(key) {
	case "User-Agent":
		return "Grpcgateway-User-Agent", true

	case
		"Content-Length",
		"Grpcgateway-User-Agent",
		"Host",
		"Connection",
		"Keep-Alive",
		"Proxy-Connection",
		"Transfer-Encoding",
		"Upgrade":
		return "", false

	default:
		return key, true
	}
}
