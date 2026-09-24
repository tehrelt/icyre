package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/platform/httpserver"
	"github.com/tehrelt/icyre/services/catalog/internal/application"
	"github.com/tehrelt/icyre/services/catalog/internal/domain"
	"github.com/tehrelt/icyre/services/catalog/internal/ports"
)

// stubCatalog returns canned results; each test sets what it needs.
type stubCatalog struct {
	artist         domain.Artist
	album          domain.Album
	track          domain.Track
	page           application.AlbumPage
	err            error
	gotCreateTrack application.CreateTrack
	gotUpdate      application.UpdateTrack
	gotList        application.ListArtistAlbums
	gotAlbums      application.ListAlbums
	gotIDs         []uuid.UUID
}

func (s *stubCatalog) CreateArtist(_ context.Context, cmd application.CreateArtist) (domain.Artist, error) {
	if s.err != nil {
		return domain.Artist{}, s.err
	}
	return domain.NewArtist(uuid.Must(uuid.NewV7()), cmd.Name, time.Now())
}
func (s *stubCatalog) GetArtist(context.Context, uuid.UUID) (domain.Artist, error) {
	return s.artist, s.err
}
func (s *stubCatalog) ListArtists(_ context.Context, ids []uuid.UUID) ([]domain.Artist, error) {
	s.gotIDs = ids
	return []domain.Artist{s.artist}, s.err
}
func (s *stubCatalog) ListAlbums(_ context.Context, q application.ListAlbums) (application.AlbumPage, error) {
	s.gotAlbums = q
	return s.page, s.err
}
func (s *stubCatalog) ListArtistAlbums(_ context.Context, q application.ListArtistAlbums) (application.AlbumPage, error) {
	s.gotList = q
	return s.page, s.err
}
func (s *stubCatalog) CreateAlbum(context.Context, application.CreateAlbum) (domain.Album, error) {
	return s.album, s.err
}
func (s *stubCatalog) GetAlbum(context.Context, uuid.UUID) (domain.Album, error) {
	return s.album, s.err
}
func (s *stubCatalog) ListAlbumTracks(context.Context, uuid.UUID) ([]domain.Track, error) {
	return []domain.Track{s.track}, s.err
}
func (s *stubCatalog) CreateTrack(_ context.Context, cmd application.CreateTrack) (domain.Track, error) {
	s.gotCreateTrack = cmd
	return s.track, s.err
}
func (s *stubCatalog) GetTrack(context.Context, uuid.UUID) (domain.Track, error) {
	return s.track, s.err
}
func (s *stubCatalog) UpdateTrack(_ context.Context, cmd application.UpdateTrack) (domain.Track, error) {
	s.gotUpdate = cmd
	return s.track, s.err
}
func (s *stubCatalog) ListGenres(context.Context) ([]domain.Genre, error) { return nil, s.err }

func serve(t *testing.T, app Catalog, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	NewHandler(app, slog.New(slog.NewTextHandler(io.Discard, nil))).Register(mux)
	h := httpserver.Chain(mux, httpserver.RequestID())

	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) httpserver.ErrorPayload {
	t.Helper()
	var body httpserver.ErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("not an error envelope: %s", rec.Body)
	}
	if body.Error.RequestID == "" {
		t.Error("error envelope misses requestId")
	}
	return body.Error
}

func sampleTrack() domain.Track {
	tr, _ := domain.NewTrack(domain.NewTrackParams{
		ID: uuid.Must(uuid.NewV7()), AlbumID: uuid.Must(uuid.NewV7()), ArtistIDs: []uuid.UUID{uuid.Must(uuid.NewV7())},
		Title: "Glass Tides", Duration: 227 * time.Second, TrackNumber: 2,
	}, time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC))
	return tr
}

func TestCreateArtist(t *testing.T) {
	rec := serve(t, &stubCatalog{}, http.MethodPost, "/api/v1/artists", `{"name":"Nova Hale"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	var got artistResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Name != "Nova Hale" || rec.Header().Get("Location") != "/api/v1/artists/"+got.ID {
		t.Fatalf("body = %+v, location = %s", got, rec.Header().Get("Location"))
	}
}

func TestGetTrackJSONShape(t *testing.T) {
	tr := sampleTrack()
	rec := serve(t, &stubCatalog{track: tr}, http.MethodGet, "/api/v1/tracks/"+tr.ID.String(), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{`"durationMs":227000`, `"status":"DRAFT"`, `"trackNumber":2`, `"isrc":null`, `"createdAt":"2026-09-24T12:00:00Z"`} {
		if !strings.Contains(body, want) {
			t.Errorf("body %s misses %s", body, want)
		}
	}
}

func TestErrorMapping(t *testing.T) {
	trackURL := "/api/v1/tracks/" + uuid.NewString()
	cases := []struct {
		name   string
		err    error
		method string
		path   string
		body   string
		status int
		code   string
	}{
		{"not found", domain.ErrTrackNotFound, http.MethodGet, trackURL, "", 404, codeTrackNotFound},
		{"wrapped not found", fmt.Errorf("x: %w", domain.ErrArtistNotFound), http.MethodGet, "/api/v1/artists/" + uuid.NewString(), "", 404, codeArtistNotFound},
		{"validation", &domain.ValidationError{Fields: map[string]string{"name": "must not be empty"}}, http.MethodPost, "/api/v1/artists", `{"name":""}`, 422, httpserver.CodeValidation},
		{"reference", &domain.ReferenceError{Field: "albumId", Err: domain.ErrAlbumNotFound}, http.MethodPost, "/api/v1/tracks", `{"albumId":"` + uuid.NewString() + `","title":"x","durationMs":1000,"trackNumber":1}`, 422, codeAlbumNotFound},
		{"position taken", domain.ErrTrackPositionTaken, http.MethodPost, "/api/v1/tracks", `{"albumId":"` + uuid.NewString() + `","title":"x","durationMs":1000,"trackNumber":1}`, 409, codeTrackPositionTaken},
		{"transition", fmt.Errorf("%w: READY → DRAFT", domain.ErrInvalidStatusTransition), http.MethodPatch, trackURL, `{"status":"draft"}`, 422, codeInvalidStatusTransition},
		{"internal", errors.New("db exploded"), http.MethodGet, trackURL, "", 500, httpserver.CodeInternal},
		{"bad uuid", nil, http.MethodGet, "/api/v1/tracks/not-a-uuid", "", 400, httpserver.CodeBadRequest},
		{"malformed json", nil, http.MethodPost, "/api/v1/artists", `{"name":`, 400, httpserver.CodeBadRequest},
		{"unknown field", nil, http.MethodPost, "/api/v1/artists", `{"name":"x","genre":"y"}`, 400, httpserver.CodeBadRequest},
		{"bad album id", nil, http.MethodPost, "/api/v1/tracks", `{"albumId":"nope","title":"x"}`, 422, httpserver.CodeValidation},
		{"bad status", nil, http.MethodPatch, trackURL, `{"status":"LIVE"}`, 422, httpserver.CodeValidation},
		{"bad date", nil, http.MethodPost, "/api/v1/albums", `{"title":"x","albumType":"ALBUM","releaseDate":"06.03.2026","artistIds":[]}`, 422, httpserver.CodeValidation},
		{"bad cursor", nil, http.MethodGet, "/api/v1/artists/" + uuid.NewString() + "/albums?cursor=not!base64", "", 400, codeInvalidCursor},
		{"bad limit", nil, http.MethodGet, "/api/v1/artists/" + uuid.NewString() + "/albums?limit=1000", "", 422, httpserver.CodeValidation},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := serve(t, &stubCatalog{err: c.err, track: sampleTrack()}, c.method, c.path, c.body)
			if rec.Code != c.status {
				t.Fatalf("status = %d, want %d: %s", rec.Code, c.status, rec.Body)
			}
			if got := errorCode(t, rec); got.Code != c.code {
				t.Fatalf("code = %s, want %s", got.Code, c.code)
			}
			if c.status == 500 && strings.Contains(rec.Body.String(), "exploded") {
				t.Fatal("internal error details leaked to the client")
			}
		})
	}
}

func TestUpdateTrackPassesPartialChanges(t *testing.T) {
	stub := &stubCatalog{track: sampleTrack()}
	rec := serve(t, stub, http.MethodPatch, "/api/v1/tracks/"+uuid.NewString(), `{"status":"processing","explicit":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	c := stub.gotUpdate.Changes
	if c.Title != nil || c.Explicit == nil || !*c.Explicit || c.Status == nil || *c.Status != domain.TrackStatusProcessing {
		t.Fatalf("changes = %+v", c)
	}
}

func TestListArtistAlbumsCursorRoundTrip(t *testing.T) {
	next := ports.AlbumCursor{ReleaseDate: time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC), ID: uuid.Must(uuid.NewV7())}
	stub := &stubCatalog{page: application.AlbumPage{Next: &next}}
	artist := uuid.NewString()

	rec := serve(t, stub, http.MethodGet, "/api/v1/artists/"+artist+"/albums?limit=5", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	var body listResponse[albumResponse]
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Pagination == nil || !body.Pagination.HasMore || body.Pagination.NextCursor == nil || body.Data == nil {
		t.Fatalf("pagination = %+v, data = %v", body.Pagination, body.Data)
	}

	rec = serve(t, stub, http.MethodGet, "/api/v1/artists/"+artist+"/albums?cursor="+*body.Pagination.NextCursor, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if stub.gotList.After == nil || stub.gotList.After.ID != next.ID || !stub.gotList.After.ReleaseDate.Equal(next.ReleaseDate) {
		t.Fatalf("cursor did not round-trip: %+v", stub.gotList.After)
	}
}

func TestCreateTrackMapsDuration(t *testing.T) {
	stub := &stubCatalog{track: sampleTrack()}
	album := uuid.NewString()
	rec := serve(t, stub, http.MethodPost, "/api/v1/tracks", `{"albumId":"`+album+`","title":"Glass Tides","durationMs":227500,"trackNumber":2}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if stub.gotCreateTrack.Duration != 227500*time.Millisecond || stub.gotCreateTrack.AlbumID.String() != album {
		t.Fatalf("command = %+v", stub.gotCreateTrack)
	}
}

func TestBatchArtistsAndAlbumList(t *testing.T) {
	a, _ := domain.NewArtist(uuid.Must(uuid.NewV7()), "Nova Hale", time.Now())
	stub := &stubCatalog{artist: a}
	id1, id2 := uuid.NewString(), uuid.NewString()

	rec := serve(t, stub, http.MethodGet, "/api/v1/artists?ids="+id1+","+id2, "")
	if rec.Code != http.StatusOK || len(stub.gotIDs) != 2 || !strings.Contains(rec.Body.String(), `"name":"Nova Hale"`) {
		t.Fatalf("status = %d, ids = %v, body = %s", rec.Code, stub.gotIDs, rec.Body)
	}
	if rec := serve(t, stub, http.MethodGet, "/api/v1/artists?ids=nope", ""); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad id status = %d", rec.Code)
	}
	if rec := serve(t, stub, http.MethodGet, "/api/v1/artists", ""); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing ids status = %d", rec.Code)
	}

	rec = serve(t, stub, http.MethodGet, "/api/v1/albums?limit=3", "")
	if rec.Code != http.StatusOK || stub.gotAlbums.Limit != 3 || !strings.Contains(rec.Body.String(), `"pagination"`) {
		t.Fatalf("status = %d, q = %+v, body = %s", rec.Code, stub.gotAlbums, rec.Body)
	}
}
