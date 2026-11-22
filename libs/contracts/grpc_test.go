package contracts_test

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/known/timestamppb"

	catalogv1 "github.com/tehrelt/icyre/libs/contracts/gen/catalog/v1"
	playbackv1 "github.com/tehrelt/icyre/libs/contracts/gen/playback/v1"
	usersv1 "github.com/tehrelt/icyre/libs/contracts/gen/users/v1"
)

type requiredRPC struct {
	name   string
	input  protoreflect.FullName
	output protoreflect.FullName
}

type serviceContractCase struct {
	fullName        protoreflect.FullName
	fileDesc        protoreflect.FileDescriptor
	grpcDesc        grpc.ServiceDesc
	requiredMethods []requiredRPC
}

func catalogServiceCase() serviceContractCase {
	return serviceContractCase{
		fullName: "catalog.v1.CatalogService",
		fileDesc: catalogv1.File_catalog_v1_catalog_proto,
		grpcDesc: catalogv1.CatalogService_ServiceDesc,
		requiredMethods: []requiredRPC{
			{name: "GetArtist", input: "catalog.v1.GetArtistRequest", output: "catalog.v1.GetArtistResponse"},
			{name: "GetAlbum", input: "catalog.v1.GetAlbumRequest", output: "catalog.v1.GetAlbumResponse"},
			{name: "GetTrack", input: "catalog.v1.GetTrackRequest", output: "catalog.v1.GetTrackResponse"},
			{name: "BatchGetArtists", input: "catalog.v1.BatchGetArtistsRequest", output: "catalog.v1.BatchGetArtistsResponse"},
			{name: "BatchGetTracks", input: "catalog.v1.BatchGetTracksRequest", output: "catalog.v1.BatchGetTracksResponse"},
			{name: "ListAlbums", input: "catalog.v1.ListAlbumsRequest", output: "catalog.v1.ListAlbumsResponse"},
			{name: "ListArtistAlbums", input: "catalog.v1.ListArtistAlbumsRequest", output: "catalog.v1.ListArtistAlbumsResponse"},
			{name: "ListAlbumTracks", input: "catalog.v1.ListAlbumTracksRequest", output: "catalog.v1.ListAlbumTracksResponse"},
			{name: "ListGenres", input: "catalog.v1.ListGenresRequest", output: "catalog.v1.ListGenresResponse"},
		},
	}
}

func usersServiceCase() serviceContractCase {
	return serviceContractCase{
		fullName: "users.v1.UsersService",
		fileDesc: usersv1.File_users_v1_users_proto,
		grpcDesc: usersv1.UsersService_ServiceDesc,
		requiredMethods: []requiredRPC{
			{name: "GetPublicProfile", input: "users.v1.GetPublicProfileRequest", output: "users.v1.GetPublicProfileResponse"},
			{name: "GetMyProfile", input: "users.v1.GetMyProfileRequest", output: "users.v1.GetMyProfileResponse"},
			{name: "UpdateMyProfile", input: "users.v1.UpdateMyProfileRequest", output: "users.v1.UpdateMyProfileResponse"},
		},
	}
}

func playbackServiceCase() serviceContractCase {
	return serviceContractCase{
		fullName: "playback.v1.PlaybackService",
		fileDesc: playbackv1.File_playback_v1_playback_proto,
		grpcDesc: playbackv1.PlaybackService_ServiceDesc,
		requiredMethods: []requiredRPC{
			{name: "StartSession", input: "playback.v1.StartSessionRequest", output: "playback.v1.StartSessionResponse"},
			{name: "GetSession", input: "playback.v1.GetSessionRequest", output: "playback.v1.GetSessionResponse"},
			{name: "SetQueue", input: "playback.v1.SetQueueRequest", output: "playback.v1.SetQueueResponse"},
			{name: "UpdateState", input: "playback.v1.UpdateStateRequest", output: "playback.v1.UpdateStateResponse"},
			{name: "ReportEvent", input: "playback.v1.ReportEventRequest", output: "playback.v1.ReportEventResponse"},
		},
	}
}

func TestServiceDescriptorsResolve(t *testing.T) {
	cases := []serviceContractCase{
		catalogServiceCase(),
		usersServiceCase(),
		playbackServiceCase(),
	}
	for _, tc := range cases {
		t.Run(string(tc.fullName), func(t *testing.T) {
			assertServiceDescriptor(t, tc)
		})
	}
}

func assertServiceDescriptor(t *testing.T, tc serviceContractCase) {
	t.Helper()

	regFile, err := protoregistry.GlobalFiles.FindFileByPath(tc.fileDesc.Path())
	if err != nil {
		t.Fatalf("FindFileByPath(%q): %v", tc.fileDesc.Path(), err)
	}
	if regFile.Services().ByName(tc.fullName.Name()) == nil {
		t.Fatalf("service %q missing from registered file %q", tc.fullName.Name(), tc.fileDesc.Path())
	}

	desc, err := protoregistry.GlobalFiles.FindDescriptorByName(tc.fullName)
	if err != nil {
		t.Fatalf("FindDescriptorByName(%q): %v", tc.fullName, err)
	}
	sd, ok := desc.(protoreflect.ServiceDescriptor)
	if !ok {
		t.Fatalf("descriptor %q is %T, want protoreflect.ServiceDescriptor", tc.fullName, desc)
	}
	if tc.grpcDesc.ServiceName != string(tc.fullName) {
		t.Fatalf("grpc.ServiceDesc.ServiceName = %q, want %q", tc.grpcDesc.ServiceName, tc.fullName)
	}

	grpcMethods, grpcStreams := indexGRPCServiceDesc(tc.grpcDesc)
	for _, rpc := range tc.requiredMethods {
		assertRequiredRPC(t, sd, grpcMethods, grpcStreams, rpc)
	}
}

func indexGRPCServiceDesc(desc grpc.ServiceDesc) (map[string]struct{}, map[string]struct{}) {
	methods := make(map[string]struct{}, len(desc.Methods))
	for _, m := range desc.Methods {
		methods[m.MethodName] = struct{}{}
	}
	streams := make(map[string]struct{}, len(desc.Streams))
	for _, s := range desc.Streams {
		streams[s.StreamName] = struct{}{}
	}
	return methods, streams
}

func assertRequiredRPC(
	t *testing.T,
	sd protoreflect.ServiceDescriptor,
	grpcMethods map[string]struct{},
	grpcStreams map[string]struct{},
	rpc requiredRPC,
) {
	t.Helper()

	m := sd.Methods().ByName(protoreflect.Name(rpc.name))
	if m == nil {
		t.Errorf("protoreflect service %q missing required method %q", sd.FullName(), rpc.name)
		return
	}
	if m.IsStreamingClient() || m.IsStreamingServer() {
		t.Errorf("method %q.%s is streaming (client=%v, server=%v), want unary",
			sd.FullName(), rpc.name, m.IsStreamingClient(), m.IsStreamingServer())
	}
	if m.Input().FullName() != rpc.input {
		t.Errorf("method %q.%s input = %q, want %q", sd.FullName(), rpc.name, m.Input().FullName(), rpc.input)
	}
	if m.Output().FullName() != rpc.output {
		t.Errorf("method %q.%s output = %q, want %q", sd.FullName(), rpc.name, m.Output().FullName(), rpc.output)
	}
	if _, ok := grpcMethods[rpc.name]; !ok {
		t.Errorf("grpc.ServiceDesc %q missing required unary method %q", sd.FullName(), rpc.name)
	}
	if _, ok := grpcStreams[rpc.name]; ok {
		t.Errorf("grpc.ServiceDesc %q lists %q in Streams, want unary Methods only", sd.FullName(), rpc.name)
	}
}

func roundTrip[T proto.Message](t *testing.T, in T, out T) T {
	t.Helper()
	raw, err := proto.Marshal(in)
	if err != nil {
		t.Fatalf("proto.Marshal(%T): %v", in, err)
	}
	if err := proto.Unmarshal(raw, out); err != nil {
		t.Fatalf("proto.Unmarshal(%T): %v", out, err)
	}
	return out
}

func TestProto3OptionalPresenceRoundTrip(t *testing.T) {
	t.Run("UpdateMyProfileRequest", testUpdateMyProfilePresence)
	t.Run("UpdateStateRequest", testUpdateStatePresence)
	t.Run("SetQueueRequest", testSetQueuePresence)
	t.Run("ListAlbumsRequest", testListAlbumsPresence)
	t.Run("Track", testTrackPresence)
	t.Run("Session", testSessionPresence)
}

func testUpdateMyProfilePresence(t *testing.T) {
	explicitZero := &usersv1.UpdateMyProfileRequest{
		Username:    new(""),
		DisplayName: new(""),
		Bio:         new(""),
		Country:     new(""),
		Language:    new(""),
	}
	gotZero := roundTrip(t, explicitZero, &usersv1.UpdateMyProfileRequest{})
	for name, ptr := range map[string]*string{
		"Username":    gotZero.Username,
		"DisplayName": gotZero.DisplayName,
		"Bio":         gotZero.Bio,
		"Country":     gotZero.Country,
		"Language":    gotZero.Language,
	} {
		if ptr == nil || *ptr != "" {
			t.Fatalf("%s = %v, want non-nil empty string", name, ptr)
		}
	}

	omitted := &usersv1.UpdateMyProfileRequest{Bio: new("updated bio")}
	gotOmitted := roundTrip(t, omitted, &usersv1.UpdateMyProfileRequest{})
	if gotOmitted.Username != nil || gotOmitted.DisplayName != nil || gotOmitted.Country != nil || gotOmitted.Language != nil {
		t.Fatalf("expected omitted fields to remain nil, got %+v", gotOmitted)
	}
	if gotOmitted.Bio == nil || *gotOmitted.Bio != "updated bio" {
		t.Fatalf("Bio = %v, want %q", gotOmitted.Bio, "updated bio")
	}
}

func testUpdateStatePresence(t *testing.T) {
	explicitZero := &playbackv1.UpdateStateRequest{
		IsPlaying:  new(false),
		PositionMs: new(int64(0)),
		DurationMs: new(int64(0)),
		Shuffle:    new(false),
		RepeatMode: playbackv1.RepeatMode_REPEAT_MODE_UNSPECIFIED.Enum(),
		QueueIndex: new(int32(0)),
		Action:     playbackv1.PlaybackAction_PLAYBACK_ACTION_UNSPECIFIED.Enum(),
	}
	gotZero := roundTrip(t, explicitZero, &playbackv1.UpdateStateRequest{})
	if gotZero.IsPlaying == nil || *gotZero.IsPlaying || gotZero.Shuffle == nil || *gotZero.Shuffle {
		t.Fatalf("expected non-nil false booleans, got IsPlaying=%v Shuffle=%v", gotZero.IsPlaying, gotZero.Shuffle)
	}
	if gotZero.PositionMs == nil || *gotZero.PositionMs != 0 || gotZero.DurationMs == nil || *gotZero.DurationMs != 0 {
		t.Fatalf("expected non-nil 0 durations, got PositionMs=%v DurationMs=%v", gotZero.PositionMs, gotZero.DurationMs)
	}
	if gotZero.QueueIndex == nil || *gotZero.QueueIndex != 0 {
		t.Fatalf("QueueIndex = %v, want non-nil 0", gotZero.QueueIndex)
	}
	if gotZero.RepeatMode == nil || *gotZero.RepeatMode != playbackv1.RepeatMode_REPEAT_MODE_UNSPECIFIED {
		t.Fatalf("RepeatMode = %v, want non-nil REPEAT_MODE_UNSPECIFIED", gotZero.RepeatMode)
	}
	if gotZero.Action == nil || *gotZero.Action != playbackv1.PlaybackAction_PLAYBACK_ACTION_UNSPECIFIED {
		t.Fatalf("Action = %v, want non-nil PLAYBACK_ACTION_UNSPECIFIED", gotZero.Action)
	}

	gotOmitted := roundTrip(t, &playbackv1.UpdateStateRequest{PositionMs: new(int64(1234))}, &playbackv1.UpdateStateRequest{})
	if gotOmitted.IsPlaying != nil || gotOmitted.DurationMs != nil || gotOmitted.Shuffle != nil ||
		gotOmitted.RepeatMode != nil || gotOmitted.QueueIndex != nil || gotOmitted.Action != nil {
		t.Fatalf("expected omitted fields to remain nil, got %+v", gotOmitted)
	}
	if gotOmitted.PositionMs == nil || *gotOmitted.PositionMs != 1234 {
		t.Fatalf("PositionMs = %v, want 1234", gotOmitted.PositionMs)
	}
}

func testSetQueuePresence(t *testing.T) {
	explicitZero := &playbackv1.SetQueueRequest{
		Tracks:     []string{"t1", "t2"},
		StartIndex: new(int32(0)),
		Source:     "album:1",
	}
	gotZero := roundTrip(t, explicitZero, &playbackv1.SetQueueRequest{})
	if gotZero.StartIndex == nil || *gotZero.StartIndex != 0 {
		t.Fatalf("StartIndex = %v, want non-nil 0", gotZero.StartIndex)
	}

	omitted := &playbackv1.SetQueueRequest{Tracks: []string{"t1"}, Source: "album:1"}
	gotOmitted := roundTrip(t, omitted, &playbackv1.SetQueueRequest{})
	if gotOmitted.StartIndex != nil {
		t.Fatalf("StartIndex = %v, want nil", gotOmitted.StartIndex)
	}
}

func testListAlbumsPresence(t *testing.T) {
	explicitZero := &catalogv1.ListAlbumsRequest{Limit: new(int32(0)), Cursor: "cur-1"}
	gotZero := roundTrip(t, explicitZero, &catalogv1.ListAlbumsRequest{})
	if gotZero.Limit == nil || *gotZero.Limit != 0 {
		t.Fatalf("Limit = %v, want non-nil 0", gotZero.Limit)
	}

	gotOmitted := roundTrip(t, &catalogv1.ListAlbumsRequest{Cursor: "cur-1"}, &catalogv1.ListAlbumsRequest{})
	if gotOmitted.Limit != nil {
		t.Fatalf("Limit = %v, want nil", gotOmitted.Limit)
	}
}

func testTrackPresence(t *testing.T) {
	explicitEmpty := &catalogv1.Track{Id: "track-1", Isrc: new("")}
	gotEmpty := roundTrip(t, explicitEmpty, &catalogv1.Track{})
	if gotEmpty.Isrc == nil || *gotEmpty.Isrc != "" {
		t.Fatalf("Isrc = %v, want non-nil empty string", gotEmpty.Isrc)
	}

	gotOmitted := roundTrip(t, &catalogv1.Track{Id: "track-1"}, &catalogv1.Track{})
	if gotOmitted.Isrc != nil {
		t.Fatalf("Isrc = %v, want nil", gotOmitted.Isrc)
	}
}

func testSessionPresence(t *testing.T) {
	explicitEmpty := &playbackv1.Session{
		SessionId:  "sess-1",
		TrackId:    new(""),
		PlaybackId: new(""),
	}
	gotEmpty := roundTrip(t, explicitEmpty, &playbackv1.Session{})
	if gotEmpty.TrackId == nil || *gotEmpty.TrackId != "" {
		t.Fatalf("TrackId = %v, want non-nil empty string", gotEmpty.TrackId)
	}
	if gotEmpty.PlaybackId == nil || *gotEmpty.PlaybackId != "" {
		t.Fatalf("PlaybackId = %v, want non-nil empty string", gotEmpty.PlaybackId)
	}

	gotOmitted := roundTrip(t, &playbackv1.Session{SessionId: "sess-1"}, &playbackv1.Session{})
	if gotOmitted.TrackId != nil || gotOmitted.PlaybackId != nil {
		t.Fatalf("expected omitted TrackId/PlaybackId to remain nil, got %+v", gotOmitted)
	}
}

type testCatalogServer struct {
	catalogv1.UnimplementedCatalogServiceServer
}

func (testCatalogServer) GetTrack(_ context.Context, req *catalogv1.GetTrackRequest) (*catalogv1.GetTrackResponse, error) {
	return &catalogv1.GetTrackResponse{
		Track: &catalogv1.Track{
			Id:          req.GetTrackId(),
			AlbumId:     "album-1",
			ArtistIds:   []string{"artist-1"},
			Title:       "Test Track",
			DurationMs:  180000,
			TrackNumber: 1,
			DiscNumber:  1,
			Status:      catalogv1.TrackStatus_TRACK_STATUS_READY,
			Isrc:        new("USRC17607839"),
			CreatedAt:   timestamppb.Now(),
		},
	}, nil
}

type testUsersServer struct {
	usersv1.UnimplementedUsersServiceServer
}

func (testUsersServer) UpdateMyProfile(_ context.Context, req *usersv1.UpdateMyProfileRequest) (*usersv1.UpdateMyProfileResponse, error) {
	return &usersv1.UpdateMyProfileResponse{
		Profile: &usersv1.OwnProfile{
			Profile: &usersv1.PublicProfile{
				Id:          "user-1",
				Username:    req.GetUsername(),
				DisplayName: req.GetDisplayName(),
				Bio:         req.GetBio(),
			},
			Language: req.GetLanguage(),
		},
	}, nil
}

type testPlaybackServer struct {
	playbackv1.UnimplementedPlaybackServiceServer
}

func (testPlaybackServer) UpdateState(_ context.Context, req *playbackv1.UpdateStateRequest) (*playbackv1.UpdateStateResponse, error) {
	return &playbackv1.UpdateStateResponse{
		Session: &playbackv1.Session{
			SessionId:  "sess-1",
			TrackId:    new("track-1"),
			IsPlaying:  req.GetIsPlaying(),
			PositionMs: req.GetPositionMs(),
			RepeatMode: req.GetRepeatMode(),
		},
	}, nil
}

func startBufconnClient(t *testing.T) *grpc.ClientConn {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	catalogv1.RegisterCatalogServiceServer(srv, testCatalogServer{})
	usersv1.RegisterUsersServiceServer(srv, testUsersServer{})
	playbackv1.RegisterPlaybackServiceServer(srv, testPlaybackServer{})

	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(func() {
		srv.Stop()
		_ = lis.Close()
	})

	cc, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	t.Cleanup(func() { _ = cc.Close() })
	return cc
}

func TestGeneratedGRPCStubsOverBufconn(t *testing.T) {
	cc := startBufconnClient(t)
	ctx := context.Background()

	assertCatalogStub(t, ctx, catalogv1.NewCatalogServiceClient(cc))
	assertUsersStub(t, ctx, usersv1.NewUsersServiceClient(cc))
	assertPlaybackStub(t, ctx, playbackv1.NewPlaybackServiceClient(cc))
}

func assertCatalogStub(t *testing.T, ctx context.Context, client catalogv1.CatalogServiceClient) {
	t.Helper()
	trackResp, err := client.GetTrack(ctx, &catalogv1.GetTrackRequest{TrackId: "track-42"})
	if err != nil {
		t.Fatalf("CatalogService.GetTrack: %v", err)
	}
	if trackResp.GetTrack().GetId() != "track-42" || trackResp.GetTrack().GetIsrc() != "USRC17607839" {
		t.Fatalf("unexpected GetTrack response: %+v", trackResp)
	}
	if _, err := client.ListGenres(ctx, &catalogv1.ListGenresRequest{}); status.Code(err) != codes.Unimplemented {
		t.Fatalf("CatalogService.ListGenres code = %v, want Unimplemented", status.Code(err))
	}
}

func assertUsersStub(t *testing.T, ctx context.Context, client usersv1.UsersServiceClient) {
	t.Helper()
	profileResp, err := client.UpdateMyProfile(ctx, &usersv1.UpdateMyProfileRequest{
		Username: new("alice"),
		Bio:      new(""),
		Language: new("en"),
	})
	if err != nil {
		t.Fatalf("UsersService.UpdateMyProfile: %v", err)
	}
	if profileResp.GetProfile().GetProfile().GetUsername() != "alice" || profileResp.GetProfile().GetLanguage() != "en" {
		t.Fatalf("unexpected UpdateMyProfile response: %+v", profileResp)
	}
}

func assertPlaybackStub(t *testing.T, ctx context.Context, client playbackv1.PlaybackServiceClient) {
	t.Helper()
	stateResp, err := client.UpdateState(ctx, &playbackv1.UpdateStateRequest{
		IsPlaying:  new(false),
		PositionMs: new(int64(0)),
		RepeatMode: playbackv1.RepeatMode_REPEAT_MODE_ONE.Enum(),
	})
	if err != nil {
		t.Fatalf("PlaybackService.UpdateState: %v", err)
	}
	if stateResp.GetSession().GetSessionId() != "sess-1" ||
		stateResp.GetSession().GetIsPlaying() ||
		stateResp.GetSession().GetRepeatMode() != playbackv1.RepeatMode_REPEAT_MODE_ONE {
		t.Fatalf("unexpected UpdateState response: %+v", stateResp)
	}
}
