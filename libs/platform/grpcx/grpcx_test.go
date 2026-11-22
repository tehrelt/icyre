package grpcx

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/tehrelt/icyre/libs/platform/httpclient"
	"github.com/tehrelt/icyre/libs/platform/httpserver"
	"github.com/tehrelt/icyre/libs/platform/requestid"
)

const testServiceName = "grpcx.test.EchoService"

type echoServer interface {
	Unary(context.Context, *wrapperspb.StringValue) (*wrapperspb.StringValue, error)
	Stream(grpc.ServerStream) error
}

type testEchoImpl struct {
	onUnary  func(context.Context, *wrapperspb.StringValue) (*wrapperspb.StringValue, error)
	onStream func(grpc.ServerStream) error
}

func (s *testEchoImpl) Unary(ctx context.Context, req *wrapperspb.StringValue) (*wrapperspb.StringValue, error) {
	return s.onUnary(ctx, req)
}

func (s *testEchoImpl) Stream(ss grpc.ServerStream) error {
	return s.onStream(ss)
}

var testServiceDesc = grpc.ServiceDesc{
	ServiceName: testServiceName,
	HandlerType: (*echoServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Unary",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				in := new(wrapperspb.StringValue)
				if err := dec(in); err != nil {
					return nil, err
				}
				if interceptor == nil {
					return srv.(echoServer).Unary(ctx, in)
				}
				info := &grpc.UnaryServerInfo{
					Server:     srv,
					FullMethod: "/" + testServiceName + "/Unary",
				}
				return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
					return srv.(echoServer).Unary(ctx, req.(*wrapperspb.StringValue))
				})
			},
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Stream",
			ServerStreams: true,
			ClientStreams: true,
			Handler: func(srv any, stream grpc.ServerStream) error {
				return srv.(echoServer).Stream(stream)
			},
		},
	},
}

func startTestServer(t *testing.T, impl *testEchoImpl, extraServerOpts ...grpc.ServerOption) (*grpc.ClientConn, func()) {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)
	srvOpts := append(ServerOptions(), extraServerOpts...)
	srv := grpc.NewServer(srvOpts...)
	srv.RegisterService(&testServiceDesc, impl)

	go func() {
		_ = srv.Serve(lis)
	}()

	dialOpts := append(
		ClientOptions(),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	cc, err := grpc.NewClient("passthrough:///bufnet", dialOpts...)
	if err != nil {
		srv.Stop()
		_ = lis.Close()
		t.Fatalf("grpc.NewClient: %v", err)
	}

	cleanup := func() {
		_ = cc.Close()
		srv.Stop()
		_ = lis.Close()
	}
	return cc, cleanup
}

func TestTraceBaggageAndRequestIDPropagation(t *testing.T) {
	prevTP := otel.GetTracerProvider()
	prevProp := otel.GetTextMapPropagator()
	t.Cleanup(func() {
		otel.SetTracerProvider(prevTP)
		otel.SetTextMapPropagator(prevProp)
	})

	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	var (
		unaryTraceID  trace.TraceID
		unaryBaggage  string
		unaryReqID    string
		streamTraceID trace.TraceID
		streamBaggage string
		streamReqID   string
	)

	impl := &testEchoImpl{
		onUnary: func(ctx context.Context, req *wrapperspb.StringValue) (*wrapperspb.StringValue, error) {
			unaryTraceID = trace.SpanContextFromContext(ctx).TraceID()
			unaryBaggage = baggage.FromContext(ctx).Member("tenant").Value()
			unaryReqID = requestid.From(ctx)
			return wrapperspb.String(req.GetValue()), nil
		},
		onStream: func(ss grpc.ServerStream) error {
			ctx := ss.Context()
			streamTraceID = trace.SpanContextFromContext(ctx).TraceID()
			streamBaggage = baggage.FromContext(ctx).Member("tenant").Value()
			streamReqID = requestid.From(ctx)

			msg := new(wrapperspb.StringValue)
			if err := ss.RecvMsg(msg); err != nil {
				return err
			}
			return ss.SendMsg(wrapperspb.String(msg.GetValue()))
		},
	}

	cc, cleanup := startTestServer(t, impl)
	defer cleanup()

	parentCtx, parentSpan := otel.Tracer("test").Start(context.Background(), "parent")
	defer parentSpan.End()
	wantTraceID := parentSpan.SpanContext().TraceID()

	member, err := baggage.NewMember("tenant", "icyre")
	if err != nil {
		t.Fatal(err)
	}
	bag, err := baggage.New(member)
	if err != nil {
		t.Fatal(err)
	}
	ctx := baggage.ContextWithBaggage(parentCtx, bag)
	ctx = requestid.With(ctx, "req-abc:123")

	var unaryHeader metadata.MD
	out := new(wrapperspb.StringValue)
	if err := cc.Invoke(ctx, "/"+testServiceName+"/Unary", wrapperspb.String("ping"), out, grpc.Header(&unaryHeader)); err != nil {
		t.Fatalf("unary invoke: %v", err)
	}
	if out.GetValue() != "ping" {
		t.Fatalf("unary reply = %q, want ping", out.GetValue())
	}
	if unaryTraceID != wantTraceID {
		t.Fatalf("unary traceID = %s, want %s", unaryTraceID, wantTraceID)
	}
	if unaryBaggage != "icyre" {
		t.Fatalf("unary baggage = %q, want icyre", unaryBaggage)
	}
	if unaryReqID != "req-abc:123" {
		t.Fatalf("unary requestID = %q, want req-abc:123", unaryReqID)
	}
	if got := unaryHeader.Get(requestIDMetadataKey); len(got) != 1 || got[0] != "req-abc:123" {
		t.Fatalf("unary response header x-request-id = %v, want [req-abc:123]", got)
	}

	stream, err := cc.NewStream(ctx, &testServiceDesc.Streams[0], "/"+testServiceName+"/Stream")
	if err != nil {
		t.Fatalf("new stream: %v", err)
	}
	if err := stream.SendMsg(wrapperspb.String("hello")); err != nil {
		t.Fatalf("stream send: %v", err)
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("stream close send: %v", err)
	}
	streamReply := new(wrapperspb.StringValue)
	if err := stream.RecvMsg(streamReply); err != nil {
		t.Fatalf("stream recv: %v", err)
	}
	streamHeader, err := stream.Header()
	if err != nil {
		t.Fatalf("stream header: %v", err)
	}
	if err := stream.RecvMsg(new(wrapperspb.StringValue)); !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF, got %v", err)
	}

	if streamTraceID != wantTraceID {
		t.Fatalf("stream traceID = %s, want %s", streamTraceID, wantTraceID)
	}
	if streamBaggage != "icyre" {
		t.Fatalf("stream baggage = %q, want icyre", streamBaggage)
	}
	if streamReqID != "req-abc:123" {
		t.Fatalf("stream requestID = %q, want req-abc:123", streamReqID)
	}
	if got := streamHeader.Get(requestIDMetadataKey); len(got) != 1 || got[0] != "req-abc:123" {
		t.Fatalf("stream response header x-request-id = %v, want [req-abc:123]", got)
	}

	// Expect 1 client + 1 server span per RPC (4 total), no duplicate interceptor spans.
	ended := sr.Ended()
	if len(ended) != 4 {
		t.Fatalf("ended spans = %d, want 4 (no duplicate spans)", len(ended))
	}
}

func TestClientRequestIDInterceptor(t *testing.T) {
	t.Run("prefers valid context requestid and preserves unrelated outgoing metadata", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-inbound-secret", "do-not-forward"))
		ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(
			requestIDMetadataKey, "stale-id",
			"x-custom", "keep-me",
		))
		ctx = requestid.With(ctx, "ctx-id-1")
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		var capturedCtx context.Context
		err := clientUnaryRequestIDInterceptor(
			ctx,
			"/svc/Method",
			nil,
			nil,
			nil,
			func(c context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
				capturedCtx = c
				return nil
			},
		)
		if err != nil {
			t.Fatal(err)
		}

		md, ok := metadata.FromOutgoingContext(capturedCtx)
		if !ok {
			t.Fatal("missing outgoing metadata")
		}
		if got := md.Get(requestIDMetadataKey); len(got) != 1 || got[0] != "ctx-id-1" {
			t.Fatalf("x-request-id = %v, want [ctx-id-1]", got)
		}
		if got := md.Get("x-custom"); len(got) != 1 || got[0] != "keep-me" {
			t.Fatalf("x-custom = %v, want [keep-me]", got)
		}
		if got := md.Get("x-inbound-secret"); len(got) != 0 {
			t.Fatalf("inbound metadata leaked to outgoing: %v", got)
		}
		if _, hasDeadline := capturedCtx.Deadline(); !hasDeadline {
			t.Fatal("deadline was not preserved")
		}
	})

	t.Run("falls back to single valid outgoing metadata when context id missing or invalid", func(t *testing.T) {
		ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs(requestIDMetadataKey, "out-id:42"))
		ctx = requestid.With(ctx, "invalid id with spaces")

		var capturedCtx context.Context
		_, err := clientStreamRequestIDInterceptor(
			ctx,
			&grpc.StreamDesc{},
			nil,
			"/svc/Stream",
			func(c context.Context, _ *grpc.StreamDesc, _ *grpc.ClientConn, _ string, _ ...grpc.CallOption) (grpc.ClientStream, error) {
				capturedCtx = c
				return nil, nil
			},
		)
		if err != nil {
			t.Fatal(err)
		}

		md, _ := metadata.FromOutgoingContext(capturedCtx)
		if got := md.Get(requestIDMetadataKey); len(got) != 1 || got[0] != "out-id:42" {
			t.Fatalf("x-request-id = %v, want [out-id:42]", got)
		}
	})

	t.Run("replaces duplicate or invalid outgoing ids with generated uuid", func(t *testing.T) {
		ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs(
			requestIDMetadataKey, "first-id",
			requestIDMetadataKey, "second-id",
		))

		var capturedCtx context.Context
		err := clientUnaryRequestIDInterceptor(
			ctx,
			"/svc/Method",
			nil,
			nil,
			nil,
			func(c context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
				capturedCtx = c
				return nil
			},
		)
		if err != nil {
			t.Fatal(err)
		}

		md, _ := metadata.FromOutgoingContext(capturedCtx)
		got := md.Get(requestIDMetadataKey)
		if len(got) != 1 {
			t.Fatalf("x-request-id count = %d, want 1 (%v)", len(got), got)
		}
		if _, err := uuid.Parse(got[0]); err != nil {
			t.Fatalf("generated id %q is not a valid UUID: %v", got[0], err)
		}
	})
}

func TestServerRequestIDValidation(t *testing.T) {
	cases := []struct {
		name      string
		incoming  []string
		wantExact string
		wantUUID  bool
	}{
		{
			name:      "valid opaque ascii with colon",
			incoming:  []string{"gw:req_1.2-3"},
			wantExact: "gw:req_1.2-3",
		},
		{
			name:      "valid max length 128",
			incoming:  []string{strings.Repeat("a", 128)},
			wantExact: strings.Repeat("a", 128),
		},
		{
			name:     "missing header generates uuid",
			incoming: nil,
			wantUUID: true,
		},
		{
			name:     "empty header generates uuid",
			incoming: []string{""},
			wantUUID: true,
		},
		{
			name:     "too long 129 chars generates uuid",
			incoming: []string{strings.Repeat("a", 129)},
			wantUUID: true,
		},
		{
			name:     "invalid characters generate uuid",
			incoming: []string{"bad id/with$chars"},
			wantUUID: true,
		},
		{
			name:     "multiple values generate uuid",
			incoming: []string{"id-1", "id-2"},
			wantUUID: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotUnaryID string
			var gotStreamID string

			impl := &testEchoImpl{
				onUnary: func(ctx context.Context, req *wrapperspb.StringValue) (*wrapperspb.StringValue, error) {
					gotUnaryID = requestid.From(ctx)
					return wrapperspb.String(gotUnaryID), nil
				},
				onStream: func(ss grpc.ServerStream) error {
					gotStreamID = requestid.From(ss.Context())
					return ss.SendMsg(wrapperspb.String(gotStreamID))
				},
			}

			// Dial without ClientOptions so we can send raw incoming metadata directly to the server.
			lis := bufconn.Listen(1024 * 1024)
			srv := grpc.NewServer(ServerOptions()...)
			srv.RegisterService(&testServiceDesc, impl)
			go func() { _ = srv.Serve(lis) }()
			defer func() {
				srv.Stop()
				_ = lis.Close()
			}()

			cc, err := grpc.NewClient(
				"passthrough:///bufnet",
				grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
					return lis.DialContext(ctx)
				}),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = cc.Close() }()

			ctx := context.Background()
			if tc.incoming != nil {
				pairs := make([]string, 0, len(tc.incoming)*2)
				for _, v := range tc.incoming {
					pairs = append(pairs, requestIDMetadataKey, v)
				}
				ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(pairs...))
			}

			var unaryHeader metadata.MD
			out := new(wrapperspb.StringValue)
			if err := cc.Invoke(ctx, "/"+testServiceName+"/Unary", wrapperspb.String("x"), out, grpc.Header(&unaryHeader)); err != nil {
				t.Fatalf("unary invoke: %v", err)
			}

			assertID := func(kind, id string, hdr metadata.MD) {
				t.Helper()
				if tc.wantExact != "" && id != tc.wantExact {
					t.Fatalf("%s id = %q, want %q", kind, id, tc.wantExact)
				}
				if tc.wantUUID {
					if _, err := uuid.Parse(id); err != nil {
						t.Fatalf("%s id %q is not a valid UUID: %v", kind, id, err)
					}
				}
				if got := hdr.Get(requestIDMetadataKey); len(got) != 1 || got[0] != id {
					t.Fatalf("%s response header = %v, want [%s]", kind, got, id)
				}
			}

			assertID("unary", gotUnaryID, unaryHeader)

			stream, err := cc.NewStream(ctx, &testServiceDesc.Streams[0], "/"+testServiceName+"/Stream")
			if err != nil {
				t.Fatalf("new stream: %v", err)
			}
			if err := stream.RecvMsg(new(wrapperspb.StringValue)); err != nil {
				t.Fatalf("stream recv: %v", err)
			}
			streamHeader, err := stream.Header()
			if err != nil {
				t.Fatalf("stream header: %v", err)
			}
			assertID("stream", gotStreamID, streamHeader)
		})
	}
}

func TestHTTPContextToGRPCAndDownstreamHop(t *testing.T) {
	prevTP := otel.GetTracerProvider()
	prevProp := otel.GetTextMapPropagator()
	t.Cleanup(func() {
		otel.SetTracerProvider(prevTP)
		otel.SetTextMapPropagator(prevProp)
	})

	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	const wantReqID = "http-req-42"
	const wantTenant = "icyre"

	type hopCapture struct {
		traceID trace.TraceID
		baggage string
		reqID   string
	}
	var (
		entryCapture          hopCapture
		grpcHop1Capture       hopCapture
		downstreamGRPCCapture hopCapture
		downstreamHTTPCapture hopCapture
	)

	downstreamHTTP := httptest.NewServer(httpserver.Chain(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			downstreamHTTPCapture = hopCapture{
				traceID: trace.SpanContextFromContext(r.Context()).TraceID(),
				baggage: baggage.FromContext(r.Context()).Member("tenant").Value(),
				reqID:   requestid.From(r.Context()),
			}
			w.WriteHeader(http.StatusOK)
		}),
		httpserver.RequestID(),
		httpserver.Tracing("downstream-http"),
	))
	defer downstreamHTTP.Close()

	downstreamGRPCConn, cleanupDownstreamGRPC := startTestServer(t, &testEchoImpl{
		onUnary: func(ctx context.Context, req *wrapperspb.StringValue) (*wrapperspb.StringValue, error) {
			downstreamGRPCCapture = hopCapture{
				traceID: trace.SpanContextFromContext(ctx).TraceID(),
				baggage: baggage.FromContext(ctx).Member("tenant").Value(),
				reqID:   requestid.From(ctx),
			}
			return wrapperspb.String("downstream:" + req.GetValue()), nil
		},
	})
	defer cleanupDownstreamGRPC()

	httpClient := httpclient.New(httpclient.Config{Timeout: 2 * time.Second})

	hop1GRPCConn, cleanupHop1GRPC := startTestServer(t, &testEchoImpl{
		onUnary: func(ctx context.Context, req *wrapperspb.StringValue) (*wrapperspb.StringValue, error) {
			grpcHop1Capture = hopCapture{
				traceID: trace.SpanContextFromContext(ctx).TraceID(),
				baggage: baggage.FromContext(ctx).Member("tenant").Value(),
				reqID:   requestid.From(ctx),
			}

			downstreamOut := new(wrapperspb.StringValue)
			if err := downstreamGRPCConn.Invoke(ctx, "/"+testServiceName+"/Unary", req, downstreamOut); err != nil {
				return nil, err
			}

			httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, downstreamHTTP.URL+"/ping", nil)
			if err != nil {
				return nil, err
			}
			resp, err := httpClient.Do(httpReq)
			if err != nil {
				return nil, err
			}
			_ = resp.Body.Close()

			return downstreamOut, nil
		},
	})
	defer cleanupHop1GRPC()

	entryHTTP := httptest.NewServer(httpserver.Chain(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			member, err := baggage.NewMember("tenant", wantTenant)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			bag, err := baggage.New(member)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			ctx = baggage.ContextWithBaggage(ctx, bag)

			entryCapture = hopCapture{
				traceID: trace.SpanContextFromContext(ctx).TraceID(),
				baggage: baggage.FromContext(ctx).Member("tenant").Value(),
				reqID:   requestid.From(ctx),
			}

			out := new(wrapperspb.StringValue)
			if err := hop1GRPCConn.Invoke(ctx, "/"+testServiceName+"/Unary", wrapperspb.String("hello"), out); err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			_, _ = w.Write([]byte(out.GetValue()))
		}),
		httpserver.RequestID(),
		httpserver.Tracing("edge-http"),
	))
	defer entryHTTP.Close()

	req, err := http.NewRequest(http.MethodGet, entryHTTP.URL+"/edge", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(requestid.Header, wantReqID)

	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("entry HTTP request: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("entry HTTP status = %d, body = %s", resp.StatusCode, string(body))
	}
	if string(body) != "downstream:hello" {
		t.Fatalf("entry HTTP body = %q, want %q", string(body), "downstream:hello")
	}
	if got := resp.Header.Get(requestid.Header); got != wantReqID {
		t.Fatalf("entry HTTP response %s = %q, want %q", requestid.Header, got, wantReqID)
	}

	if !entryCapture.traceID.IsValid() {
		t.Fatal("entry HTTP span traceID is invalid")
	}

	for name, got := range map[string]hopCapture{
		"entry-http":      entryCapture,
		"grpc-hop-1":      grpcHop1Capture,
		"downstream-grpc": downstreamGRPCCapture,
		"downstream-http": downstreamHTTPCapture,
	} {
		if got.traceID != entryCapture.traceID {
			t.Fatalf("%s traceID = %s, want %s", name, got.traceID, entryCapture.traceID)
		}
		if got.baggage != wantTenant {
			t.Fatalf("%s baggage = %q, want %q", name, got.baggage, wantTenant)
		}
		if got.reqID != wantReqID {
			t.Fatalf("%s requestID = %q, want %q", name, got.reqID, wantReqID)
		}
	}

	ended := sr.Ended()
	if len(ended) == 0 {
		t.Fatal("expected recorded spans, got 0")
	}
	for _, sp := range ended {
		if sp.SpanContext().TraceID() != entryCapture.traceID {
			t.Fatalf("span %q traceID = %s, want %s", sp.Name(), sp.SpanContext().TraceID(), entryCapture.traceID)
		}
	}
}
