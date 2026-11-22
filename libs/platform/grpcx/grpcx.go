// Package grpcx provides composable gRPC client and server options for
// OpenTelemetry trace/baggage propagation and X-Request-ID correlation.
package grpcx

import (
	"context"
	"regexp"

	"github.com/google/uuid"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/tehrelt/icyre/libs/platform/requestid"
)

const requestIDMetadataKey = "x-request-id"

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

// ClientOptions returns composable gRPC dial options that propagate W3C trace
// context, baggage, and x-request-id across unary and streaming RPCs.
func ClientOptions() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithChainUnaryInterceptor(clientUnaryRequestIDInterceptor),
		grpc.WithChainStreamInterceptor(clientStreamRequestIDInterceptor),
	}
}

// ServerOptions returns composable gRPC server options that extract W3C trace
// context, baggage, and x-request-id across unary and streaming RPCs.
func ServerOptions() []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(serverUnaryRequestIDInterceptor),
		grpc.ChainStreamInterceptor(serverStreamRequestIDInterceptor),
	}
}

func clientUnaryRequestIDInterceptor(
	ctx context.Context,
	method string,
	req, reply any,
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	return invoker(withClientRequestID(ctx), method, req, reply, cc, opts...)
}

func clientStreamRequestIDInterceptor(
	ctx context.Context,
	desc *grpc.StreamDesc,
	cc *grpc.ClientConn,
	method string,
	streamer grpc.Streamer,
	opts ...grpc.CallOption,
) (grpc.ClientStream, error) {
	return streamer(withClientRequestID(ctx), desc, cc, method, opts...)
}

func serverUnaryRequestIDInterceptor(
	ctx context.Context,
	req any,
	_ *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	return handler(withServerRequestID(ctx), req)
}

func serverStreamRequestIDInterceptor(
	srv any,
	ss grpc.ServerStream,
	_ *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	wrapped := &wrappedServerStream{
		ServerStream: ss,
		ctx:          withServerRequestID(ss.Context()),
	}
	return handler(srv, wrapped)
}

func withClientRequestID(ctx context.Context) context.Context {
	id := resolveClientRequestID(ctx)
	var outMD metadata.MD
	if md, ok := metadata.FromOutgoingContext(ctx); ok && md != nil {
		outMD = md.Copy()
	} else {
		outMD = metadata.MD{}
	}
	outMD.Set(requestIDMetadataKey, id)
	return metadata.NewOutgoingContext(ctx, outMD)
}

func resolveClientRequestID(ctx context.Context) string {
	if id := requestid.From(ctx); validRequestID.MatchString(id) {
		return id
	}
	if md, ok := metadata.FromOutgoingContext(ctx); ok {
		if vals := md.Get(requestIDMetadataKey); len(vals) == 1 && validRequestID.MatchString(vals[0]) {
			return vals[0]
		}
	}
	return uuid.NewString()
}

func withServerRequestID(ctx context.Context) context.Context {
	id := resolveServerRequestID(ctx)
	ctx = requestid.With(ctx, id)
	_ = grpc.SetHeader(ctx, metadata.Pairs(requestIDMetadataKey, id))
	return ctx
}

func resolveServerRequestID(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get(requestIDMetadataKey); len(vals) == 1 && validRequestID.MatchString(vals[0]) {
			return vals[0]
		}
	}
	return uuid.NewString()
}

type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}
