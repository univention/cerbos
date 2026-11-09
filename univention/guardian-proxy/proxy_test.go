// SPDX-FileCopyrightText: 2026 Univention GmbH
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	enginev1 "github.com/cerbos/cerbos/api/genpb/cerbos/engine/v1"
	requestv1 "github.com/cerbos/cerbos/api/genpb/cerbos/request/v1"
	responsev1 "github.com/cerbos/cerbos/api/genpb/cerbos/response/v1"
	svcv1 "github.com/cerbos/cerbos/api/genpb/cerbos/svc/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

const principalJSON = `{"id":"anton","roles":["user"],"attr":{"guardianRoles":["ucsschool:users:teacher&ucsschool:ou=school1","other"]}}`

// fakeCerbos records the principal and metadata of the last decision request.
type fakeCerbos struct {
	svcv1.UnimplementedCerbosServiceServer
	mu        sync.Mutex
	principal *enginev1.Principal
	md        metadata.MD
}

func (f *fakeCerbos) record(ctx context.Context, p *enginev1.Principal) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.principal = p
	f.md, _ = metadata.FromIncomingContext(ctx)
}

func (f *fakeCerbos) last() (*enginev1.Principal, metadata.MD) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.principal, f.md
}

func (f *fakeCerbos) CheckResourceSet(ctx context.Context, req *requestv1.CheckResourceSetRequest) (*responsev1.CheckResourceSetResponse, error) {
	f.record(ctx, req.GetPrincipal())
	return &responsev1.CheckResourceSetResponse{RequestId: req.GetRequestId()}, nil
}

func (f *fakeCerbos) CheckResourceBatch(ctx context.Context, req *requestv1.CheckResourceBatchRequest) (*responsev1.CheckResourceBatchResponse, error) {
	f.record(ctx, req.GetPrincipal())
	return &responsev1.CheckResourceBatchResponse{RequestId: req.GetRequestId()}, nil
}

func (f *fakeCerbos) CheckResources(ctx context.Context, req *requestv1.CheckResourcesRequest) (*responsev1.CheckResourcesResponse, error) {
	f.record(ctx, req.GetPrincipal())
	return &responsev1.CheckResourcesResponse{RequestId: req.GetRequestId()}, nil
}

func (f *fakeCerbos) PlanResources(ctx context.Context, req *requestv1.PlanResourcesRequest) (*responsev1.PlanResourcesResponse, error) {
	f.record(ctx, req.GetPrincipal())
	return &responsev1.PlanResourcesResponse{RequestId: req.GetRequestId()}, nil
}

type errMapper struct{}

func (errMapper) MapPrincipal(context.Context, *enginev1.Principal) error {
	return errors.New("unknown guardian role")
}

type testEnv struct {
	upstream     *fakeCerbos
	proxy        *httptest.Server
	proxyGRPC    *grpc.ClientConn
	httpRequests []string
}

func newTestEnv(t *testing.T, mapper PrincipalMapper) *testEnv {
	t.Helper()
	env := &testEnv{upstream: &fakeCerbos{}}

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	grpcSrv := grpc.NewServer()
	svcv1.RegisterCerbosServiceServer(grpcSrv, env.upstream)
	healthpb.RegisterHealthServer(grpcSrv, health.NewServer())
	go func() { _ = grpcSrv.Serve(lis) }()
	t.Cleanup(grpcSrv.Stop)

	cerbosHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		env.httpRequests = append(env.httpRequests, r.URL.Path)
		_, _ = w.Write([]byte(`{"status":"SERVING"}`))
	}))
	t.Cleanup(cerbosHTTP.Close)
	httpURL, err := url.Parse(cerbosHTTP.URL)
	require.NoError(t, err)

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	p := &proxy{upstream: svcv1.NewCerbosServiceClient(conn), mapper: mapper}
	handler, err := newHandler(t.Context(), p, httpURL)
	require.NoError(t, err)
	env.proxy = httptest.NewServer(handler)
	t.Cleanup(env.proxy.Close)

	proxyLis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	proxySrv := newGRPCServer(p, healthpb.NewHealthClient(conn))
	go func() { _ = proxySrv.Serve(proxyLis) }()
	t.Cleanup(proxySrv.Stop)
	env.proxyGRPC, err = grpc.NewClient(proxyLis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = env.proxyGRPC.Close() })

	return env
}

func (env *testEnv) post(t *testing.T, path, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, env.proxy.URL+path, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("User-Agent", "guardian-test")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

var decisionRequests = map[string]string{
	"/api/check":                `{"requestId":"1","actions":["view"],"principal":` + principalJSON + `,"resource":{"kind":"doc","instances":{"1":{}}}}`,
	"/api/check_resource_batch": `{"requestId":"1","principal":` + principalJSON + `,"resources":[{"actions":["view"],"resource":{"kind":"doc","id":"1"}}]}`,
	"/api/check/resources":      `{"requestId":"1","principal":` + principalJSON + `,"resources":[{"actions":["view"],"resource":{"kind":"doc","id":"1"}}]}`,
	"/api/plan/resources":       `{"requestId":"1","action":"view","principal":` + principalJSON + `,"resource":{"kind":"doc"}}`,
}

func TestDecisionRequestsAreMapped(t *testing.T) {
	env := newTestEnv(t, staticMapper{roles: map[string]string{"ucsschool:users:teacher&ucsschool:ou=school1": "teacher"}})

	for path, body := range decisionRequests {
		t.Run(path, func(t *testing.T) {
			resp := env.post(t, path, body)
			require.Equal(t, http.StatusOK, resp.StatusCode)

			principal, md := env.upstream.last()
			require.Equal(t, []string{"user", "teacher"}, principal.GetRoles())
			require.Len(t, principal.GetAttr()[guardianRolesAttr].GetListValue().GetValues(), 2)
			require.Equal(t, []string{"guardian-test"}, md.Get("grpcgateway-user-agent"))
			require.Equal(t, []string{"127.0.0.1"}, md.Get("x-forwarded-for"))
		})
	}
}

func TestMapperErrorFailsClosed(t *testing.T) {
	env := newTestEnv(t, errMapper{})

	for path, body := range decisionRequests {
		t.Run(path, func(t *testing.T) {
			resp := env.post(t, path, body)
			require.Equal(t, http.StatusBadRequest, resp.StatusCode)
			principal, _ := env.upstream.last()
			require.Nil(t, principal)
		})
	}
}

func TestOtherPaths(t *testing.T) {
	env := newTestEnv(t, staticMapper{})

	require.Equal(t, http.StatusOK, env.post(t, "/_cerbos/health", "").StatusCode)
	require.Equal(t, http.StatusNotFound, env.post(t, "/access/v1/evaluation", "{}").StatusCode)
	require.Equal(t, []string{"/_cerbos/health"}, env.httpRequests)
}

func TestStaticMapperRejectsNonStringRoles(t *testing.T) {
	env := newTestEnv(t, staticMapper{})

	resp := env.post(t, "/api/check/resources", `{"principal":{"id":"anton","roles":["user"],"attr":{"guardianRoles":[1]}}}`)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func grpcPrincipal(t *testing.T) *enginev1.Principal {
	t.Helper()
	roles, err := structpb.NewValue([]any{"ucsschool:users:teacher&ucsschool:ou=school1"})
	require.NoError(t, err)
	return &enginev1.Principal{Id: "anton", Roles: []string{"user"}, Attr: map[string]*structpb.Value{guardianRolesAttr: roles}}
}

func TestGRPCDecisionRequestsAreMapped(t *testing.T) {
	env := newTestEnv(t, staticMapper{roles: map[string]string{"ucsschool:users:teacher&ucsschool:ou=school1": "teacher"}})
	client := svcv1.NewCerbosServiceClient(env.proxyGRPC)
	ctx := metadata.AppendToOutgoingContext(t.Context(), "x-test", "1")

	_, err := client.CheckResources(ctx, &requestv1.CheckResourcesRequest{Principal: grpcPrincipal(t)})
	require.NoError(t, err)
	principal, md := env.upstream.last()
	require.Equal(t, []string{"user", "teacher"}, principal.GetRoles())
	require.Equal(t, []string{"1"}, md.Get("x-test"))
	require.Equal(t, []string{"127.0.0.1"}, md.Get("x-forwarded-for"))

	_, err = client.PlanResources(ctx, &requestv1.PlanResourcesRequest{Principal: grpcPrincipal(t)})
	require.NoError(t, err)
	principal, _ = env.upstream.last()
	require.Equal(t, []string{"user", "teacher"}, principal.GetRoles())
}

func TestGRPCMapperErrorFailsClosed(t *testing.T) {
	env := newTestEnv(t, errMapper{})

	_, err := svcv1.NewCerbosServiceClient(env.proxyGRPC).CheckResources(t.Context(), &requestv1.CheckResourcesRequest{Principal: grpcPrincipal(t)})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	principal, _ := env.upstream.last()
	require.Nil(t, principal)
}

func TestGRPCHealthAndUnregisteredServices(t *testing.T) {
	env := newTestEnv(t, staticMapper{})

	resp, err := healthpb.NewHealthClient(env.proxyGRPC).Check(t.Context(), &healthpb.HealthCheckRequest{})
	require.NoError(t, err)
	require.Equal(t, healthpb.HealthCheckResponse_SERVING, resp.GetStatus())

	_, err = svcv1.NewCerbosAdminServiceClient(env.proxyGRPC).ListPolicies(t.Context(), &requestv1.ListPoliciesRequest{})
	require.Equal(t, codes.Unimplemented, status.Code(err))
}
