package clients

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/platform/authn"
	"github.com/tehrelt/icyre/services/admin/internal/domain"
)

func TestCatalogClient(t *testing.T) {
	trackID := uuid.New()
	albumID := uuid.New()
	artistID := uuid.New()
	opID := uuid.New()
	now := time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(authn.InternalTokenHeader) != "secret-tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/internal/v1/moderation/tracks":
			if r.URL.Query().Get("status") != "READY" || r.URL.Query().Get("limit") != "10" {
				t.Fatalf("unexpected query: %s", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{
					"id": trackID.String(), "albumId": albumID.String(), "artistIds": []string{artistID.String()},
					"title": "Track 1", "durationMs": 180000, "trackNumber": 1, "discNumber": 1,
					"explicit": false, "status": "READY", "createdAt": now, "updatedAt": now,
				}},
				"pagination": map[string]any{"nextCursor": "cur-1", "hasMore": true},
			})
		case r.Method == http.MethodPatch && r.URL.Path == "/internal/v1/moderation/tracks/"+trackID.String():
			var body moderateTrackRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.OperationID != opID.String() || !body.Blocked {
				t.Fatalf("unexpected body: %+v err=%v", body, err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"operationId": opID.String(), "trackId": trackID.String(),
				"previousStatus": "READY", "currentStatus": "BLOCKED", "changed": true, "createdAt": now,
			})
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":"TRACK_NOT_FOUND","message":"secret internal detail"}}`))
		}
	}))
	defer srv.Close()

	cl := NewCatalogClient(srv.URL, "secret-tok", srv.Client())
	page, err := cl.ListTracks(context.Background(), domain.TrackListQuery{Status: "READY", Limit: 10})
	if err != nil || len(page.Items) != 1 || !page.HasMore || *page.NextCursor != "cur-1" {
		t.Fatalf("ListTracks = %+v, err = %v", page, err)
	}

	rec, err := cl.ModerateTrack(context.Background(), opID, trackID, true)
	if err != nil || !rec.Changed || rec.CurrentStatus != "BLOCKED" {
		t.Fatalf("ModerateTrack = %+v, err = %v", rec, err)
	}

	_, err = cl.ModerateTrack(context.Background(), opID, uuid.New(), true)
	var oe *domain.OwnerError
	if !errors.As(err, &oe) || oe.Status != 404 || oe.Code != "TRACK_NOT_FOUND" || strings.Contains(oe.Message, "secret internal detail") {
		t.Fatalf("expected sanitized OwnerError, got %v", err)
	}
}

func TestAuthAndMediaIngestClientsAndErrorMapping(t *testing.T) {
	userID := uuid.New()
	opID := uuid.New()
	uploadID := uuid.New()
	trackID := uuid.New()
	now := time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(authn.InternalTokenHeader) != "tok" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/internal/v1/moderation/users":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{
					"id": userID.String(), "roles": []string{"USER"}, "blocked": false, "createdAt": now, "updatedAt": now,
				}},
				"pagination": map[string]any{"nextCursor": nil, "hasMore": false},
			})
		case r.Method == http.MethodPatch && r.URL.Path == "/internal/v1/moderation/users/"+userID.String():
			_ = json.NewEncoder(w).Encode(map[string]any{
				"operationId": opID.String(), "userId": userID.String(), "blocked": true,
				"changed": true, "revokedSessions": 2, "revocationsPending": true, "createdAt": now,
			})
		case r.Method == http.MethodGet && r.URL.Path == "/internal/v1/moderation/uploads":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{
					"id": uploadID.String(), "trackId": trackID.String(), "status": "COMPLETED",
					"createdAt": now, "expiresAt": now.Add(time.Hour),
				}},
				"pagination": map[string]any{"nextCursor": nil, "hasMore": false},
			})
		case r.URL.Path == "/internal/v1/moderation/users/conflict":
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"error":{"code":"OPERATION_CONFLICT","message":"raw"}}`))
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer srv.Close()

	authCl := NewAuthClient(srv.URL, "tok", srv.Client())
	blocked := false
	users, err := authCl.ListUsers(context.Background(), domain.UserListQuery{Blocked: &blocked, Limit: 20})
	if err != nil || len(users.Items) != 1 || users.Items[0].ID != userID {
		t.Fatalf("ListUsers = %+v, err = %v", users, err)
	}
	rec, err := authCl.ModerateUser(context.Background(), opID, userID, true)
	if err != nil || !rec.RevocationsPending || rec.RevokedSessions != 2 {
		t.Fatalf("ModerateUser = %+v, err = %v", rec, err)
	}

	ingestCl := NewMediaIngestClient(srv.URL, "tok", srv.Client())
	uploads, err := ingestCl.ListUploads(context.Background(), domain.IngestListQuery{TrackID: &trackID, Status: "COMPLETED", Limit: 10})
	if err != nil || len(uploads.Items) != 1 || uploads.Items[0].ID != uploadID {
		t.Fatalf("ListUploads = %+v, err = %v", uploads, err)
	}

	// 503 maps to ErrUpstreamUnavailable.
	if _, err := authCl.ModerateUser(context.Background(), opID, uuid.New(), true); !errors.Is(err, domain.ErrUpstreamUnavailable) {
		t.Fatalf("expected ErrUpstreamUnavailable, got %v", err)
	}

	// Bad token (403) maps to ErrUpstreamUnavailable.
	badAuth := NewAuthClient(srv.URL, "wrong", srv.Client())
	if _, err := badAuth.ListUsers(context.Background(), domain.UserListQuery{}); !errors.Is(err, domain.ErrUpstreamUnavailable) {
		t.Fatalf("expected ErrUpstreamUnavailable on 403, got %v", err)
	}

	// 409 conflict maps to OwnerError.
	conflictSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/v1/moderation/users/" + userID.String():
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"error":{"code":"OPERATION_CONFLICT"}}`))
		case "/internal/v1/moderation/tracks/" + trackID.String():
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"error":{"code":"INVALID_MODERATION_TRANSITION"}}`))
		case "/internal/v1/moderation/uploads":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"code":"INVALID_CURSOR"}}`))
		default:
			_, _ = w.Write([]byte(`{bad-json`))
		}
	}))
	defer conflictSrv.Close()

	var oe *domain.OwnerError
	if _, err := NewAuthClient(conflictSrv.URL, "tok", conflictSrv.Client()).ModerateUser(context.Background(), opID, userID, true); !errors.As(err, &oe) || oe.Code != "OPERATION_CONFLICT" {
		t.Fatalf("expected OPERATION_CONFLICT, got %v", err)
	}
	if _, err := NewCatalogClient(conflictSrv.URL, "tok", conflictSrv.Client()).ModerateTrack(context.Background(), opID, trackID, true); !errors.As(err, &oe) || oe.Code != "INVALID_MODERATION_TRANSITION" {
		t.Fatalf("expected INVALID_MODERATION_TRANSITION, got %v", err)
	}
	if _, err := NewMediaIngestClient(conflictSrv.URL, "tok", conflictSrv.Client()).ListUploads(context.Background(), domain.IngestListQuery{}); !errors.As(err, &oe) || oe.Code != "INVALID_CURSOR" {
		t.Fatalf("expected INVALID_CURSOR, got %v", err)
	}
	if _, err := NewCatalogClient(conflictSrv.URL, "tok", conflictSrv.Client()).ListTracks(context.Background(), domain.TrackListQuery{}); !errors.Is(err, domain.ErrUpstreamUnavailable) {
		t.Fatalf("expected ErrUpstreamUnavailable on bad JSON, got %v", err)
	}

	// Transport error maps to ErrUpstreamUnavailable.
	conflictSrv.Close()
	if _, err := NewCatalogClient(conflictSrv.URL, "tok", conflictSrv.Client()).ListTracks(context.Background(), domain.TrackListQuery{}); !errors.Is(err, domain.ErrUpstreamUnavailable) {
		t.Fatalf("expected ErrUpstreamUnavailable on closed server, got %v", err)
	}
}
