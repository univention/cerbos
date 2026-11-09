// SPDX-FileCopyrightText: 2026 Univention GmbH
// SPDX-License-Identifier: AGPL-3.0-only

// Command guardian-proxy serves the Cerbos HTTP API in front of Cerbos and
// maps the principal of each decision request before Cerbos evaluates it.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	svcv1 "github.com/cerbos/cerbos/api/genpb/cerbos/svc/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

type config struct {
	listen, grpcListen, cerbosGRPC, cerbosHTTP, roleMappings string
}

func main() {
	var cfg config
	flag.StringVar(&cfg.listen, "listen", ":3600", "HTTP listen address")
	flag.StringVar(&cfg.grpcListen, "grpc-listen", ":3601", "gRPC listen address")
	flag.StringVar(&cfg.cerbosGRPC, "cerbos-grpc", "localhost:3593", "Cerbos gRPC address")
	flag.StringVar(&cfg.cerbosHTTP, "cerbos-http", "http://localhost:3592", "Cerbos HTTP URL for the non-decision API")
	flag.StringVar(&cfg.roleMappings, "role-mappings", "", "JSON file mapping guardianRoles values to Cerbos roles; read at startup")
	flag.Parse()

	if err := run(cfg); err != nil {
		slog.Error("guardian-proxy failed", "error", err)
		os.Exit(1)
	}
}

func run(cfg config) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mapper, err := loadStaticMapper(cfg.roleMappings)
	if err != nil {
		return err
	}

	httpURL, err := url.Parse(cfg.cerbosHTTP)
	if err != nil {
		return err
	}

	conn, err := grpc.NewClient(cfg.cerbosGRPC, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	p := &proxy{upstream: svcv1.NewCerbosServiceClient(conn), mapper: mapper}
	handler, err := newHandler(ctx, p, httpURL)
	if err != nil {
		return err
	}

	httpLis, err := net.Listen("tcp", cfg.listen)
	if err != nil {
		return err
	}

	grpcLis, err := net.Listen("tcp", cfg.grpcListen)
	if err != nil {
		return err
	}

	httpSrv := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	grpcSrv := newGRPCServer(p, healthpb.NewHealthClient(conn))

	errc := make(chan error, 2)
	go func() { errc <- httpSrv.Serve(httpLis) }()
	go func() { errc <- grpcSrv.Serve(grpcLis) }()
	slog.Info("guardian-proxy listening", "http", cfg.listen, "grpc", cfg.grpcListen, "cerbos-grpc", cfg.cerbosGRPC, "cerbos-http", cfg.cerbosHTTP)

	select {
	case <-ctx.Done():
	case err = <-errc:
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
	grpcSrv.GracefulStop()

	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return err
}
