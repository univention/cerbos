// SPDX-FileCopyrightText: 2026 Univention GmbH
// SPDX-License-Identifier: AGPL-3.0-only

package guardianroles

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	enginev1 "github.com/cerbos/cerbos/api/genpb/cerbos/engine/v1"
	requestv1 "github.com/cerbos/cerbos/api/genpb/cerbos/request/v1"
	svcv1 "github.com/cerbos/cerbos/api/genpb/cerbos/svc/v1"
)

func call(t *testing.T, mapPrincipal func(context.Context, *enginev1.Principal) error, req any) (handlerCalled bool, err error) {
	t.Helper()
	_, err = interceptor(mapPrincipal)(context.Background(), req, &grpc.UnaryServerInfo{}, func(context.Context, any) (any, error) {
		handlerCalled = true
		return nil, nil
	})
	return handlerCalled, err
}

// principalPaths returns the paths to principal fields in msg, up to two levels deep,
// which reaches the requests wrapped by PlaygroundProxyRequest.
func principalPaths(msg protoreflect.MessageDescriptor, depth int) [][]protoreflect.FieldDescriptor {
	principalName := (&enginev1.Principal{}).ProtoReflect().Descriptor().FullName()
	var paths [][]protoreflect.FieldDescriptor
	fields := msg.Fields()
	for i := range fields.Len() {
		f := fields.Get(i)
		if f.Message() == nil || f.IsList() || f.IsMap() {
			continue
		}
		if f.Message().FullName() == principalName {
			paths = append(paths, []protoreflect.FieldDescriptor{f})
			continue
		}
		if depth > 1 {
			for _, sub := range principalPaths(f.Message(), depth-1) {
				paths = append(paths, append([]protoreflect.FieldDescriptor{f}, sub...))
			}
		}
	}
	return paths
}

// New APIs that take a principal must be added to principalOf.
func TestInterceptorCoversEveryMethodWithPrincipal(t *testing.T) {
	covered := 0
	services := svcv1.File_cerbos_svc_v1_svc_proto.Services()
	for i := range services.Len() {
		methods := services.Get(i).Methods()
		for j := range methods.Len() {
			input := methods.Get(j).Input()
			for _, path := range principalPaths(input, 2) {
				name := string(methods.Get(j).Name())
				for _, f := range path[:len(path)-1] {
					name += "/" + string(f.Name())
				}

				t.Run(name, func(t *testing.T) {
					mt, err := protoregistry.GlobalTypes.FindMessageByName(input.FullName())
					require.NoError(t, err)
					req := mt.New()
					msg := req
					for _, f := range path[:len(path)-1] {
						msg = msg.Mutable(f).Message()
					}
					principal := &enginev1.Principal{Id: "anton"}
					msg.Set(path[len(path)-1], protoreflect.ValueOfMessage(principal.ProtoReflect()))

					var got *enginev1.Principal
					handlerCalled, err := call(t, func(_ context.Context, p *enginev1.Principal) error {
						got = p
						return nil
					}, req.Interface())
					require.NoError(t, err)
					require.True(t, handlerCalled)
					require.Same(t, principal, got)
				})
				covered++
			}
		}
	}
	// 4 decision APIs, PlaygroundEvaluate and the 4 requests wrapped by PlaygroundProxy.
	require.Equal(t, 9, covered)
}

func TestInterceptorFailsClosed(t *testing.T) {
	req := &requestv1.CheckResourcesRequest{Principal: &enginev1.Principal{Id: "anton"}}
	handlerCalled, err := call(t, func(context.Context, *enginev1.Principal) error {
		return errors.New("boom")
	}, req)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	require.False(t, handlerCalled)
}

func TestInterceptorPassesOtherRequests(t *testing.T) {
	for _, req := range []any{&requestv1.ServerInfoRequest{}, &requestv1.CheckResourcesRequest{}} {
		handlerCalled, err := call(t, func(context.Context, *enginev1.Principal) error {
			t.Fatal("hook called")
			return nil
		}, req)
		require.NoError(t, err)
		require.True(t, handlerCalled)
	}
}

func TestMapPrincipalIsNoOp(t *testing.T) {
	principal := &enginev1.Principal{Id: "anton", Roles: []string{"user"}}
	want := proto.Clone(principal)
	require.NoError(t, MapPrincipal(context.Background(), principal))
	require.True(t, proto.Equal(want, principal))
}
