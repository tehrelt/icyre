// Package http exposes the Catalog use cases as the public REST API.
// Handlers only translate: decode → validate shape → call → encode.
package http

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/platform/httpserver"
	"github.com/tehrelt/icyre/services/catalog/internal/application"
	"github.com/tehrelt/icyre/services/catalog/internal/domain"
	"github.com/tehrelt/icyre/services/catalog/internal/ports"
)

// Catalog is the subset of application.Service the handlers call.
type Catalog interface {
	CreateArtist(ctx context.Context, cmd application.CreateArtist) (domain.Artist, error)
	GetArtist(ctx context.Context, id uuid.UUID) (domain.Artist, error)
	ListArtists(ctx context.Context, ids []uuid.UUID) ([]domain.Artist, error)
	ListAlbums(ctx context.Context, q application.ListAlbums) (application.AlbumPage, error)
	ListArtistAlbums(ctx context.Context, q application.ListArtistAlbums) (application.AlbumPage, error)
	CreateAlbum(ctx context.Context, cmd application.CreateAlbum) (domain.Album, error)
	GetAlbum(ctx context.Context, id uuid.UUID) (domain.Album, error)
	ListAlbumTracks(ctx context.Context, albumID uuid.UUID) ([]domain.Track, error)
	CreateTrack(ctx context.Context, cmd application.CreateTrack) (domain.Track, error)
	GetTrack(ctx context.Context, id uuid.UUID) (domain.Track, error)
	UpdateTrack(ctx context.Context, cmd application.UpdateTrack) (domain.Track, error)
	ListGenres(ctx context.Context) ([]domain.Genre, error)
}

// Handler serves /api/v1 catalog routes.
type Handler struct {
	app Catalog
	log *slog.Logger
}

// NewHandler returns a Handler.
func NewHandler(app Catalog, log *slog.Logger) *Handler {
	return &Handler{app: app, log: log}
}

// Register mounts the routes on mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/artists", h.createArtist)
	mux.HandleFunc("GET /api/v1/artists", h.listArtists)
	mux.HandleFunc("GET /api/v1/artists/{id}", h.getArtist)
	mux.HandleFunc("GET /api/v1/artists/{id}/albums", h.listArtistAlbums)
	mux.HandleFunc("POST /api/v1/albums", h.createAlbum)
	mux.HandleFunc("GET /api/v1/albums", h.listAlbums)
	mux.HandleFunc("GET /api/v1/albums/{id}", h.getAlbum)
	mux.HandleFunc("GET /api/v1/albums/{id}/tracks", h.listAlbumTracks)
	mux.HandleFunc("POST /api/v1/tracks", h.createTrack)
	mux.HandleFunc("GET /api/v1/tracks/{id}", h.getTrack)
	mux.HandleFunc("PATCH /api/v1/tracks/{id}", h.updateTrack)
	mux.HandleFunc("GET /api/v1/genres", h.listGenres)
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	writeError(w, r, h.log, err)
}

func (h *Handler) createArtist(w http.ResponseWriter, r *http.Request) {
	var req createArtistRequest
	if err := decode(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	a, err := h.app.CreateArtist(r.Context(), application.CreateArtist{Name: req.Name})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/artists/"+a.ID.String())
	httpserver.WriteJSON(w, http.StatusCreated, toArtist(a))
}

func (h *Handler) getArtist(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	a, err := h.app.GetArtist(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toArtist(a))
}

// listArtists: GET /api/v1/artists?ids=<uuid>,<uuid> — batch lookup; unknown IDs are omitted.
func (h *Handler) listArtists(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("ids")
	if raw == "" {
		h.fail(w, r, invalidFields(map[string]string{"ids": "comma-separated artist IDs are required"}))
		return
	}
	fields := map[string]string{}
	ids := parseIDs(strings.Split(raw, ","), "ids", fields)
	if len(fields) > 0 {
		h.fail(w, r, invalidFields(fields))
		return
	}
	artists, err := h.app.ListArtists(r.Context(), ids)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, listResponse[artistResponse]{Data: mapSlice(artists, toArtist)})
}

func (h *Handler) listArtistAlbums(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	limit, after, err := pageParams(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	page, err := h.app.ListArtistAlbums(r.Context(), application.ListArtistAlbums{ArtistID: id, After: after, Limit: limit})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeAlbumPage(w, page)
}

// listAlbums: GET /api/v1/albums — the whole catalogue, newest releases first.
func (h *Handler) listAlbums(w http.ResponseWriter, r *http.Request) {
	limit, after, err := pageParams(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	page, err := h.app.ListAlbums(r.Context(), application.ListAlbums{After: after, Limit: limit})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeAlbumPage(w, page)
}

func writeAlbumPage(w http.ResponseWriter, page application.AlbumPage) {
	p := &pagination{HasMore: page.Next != nil}
	if page.Next != nil {
		c := encodeCursor(*page.Next)
		p.NextCursor = &c
	}
	httpserver.WriteJSON(w, http.StatusOK, listResponse[albumResponse]{Data: mapSlice(page.Albums, toAlbum), Pagination: p})
}

func (h *Handler) createAlbum(w http.ResponseWriter, r *http.Request) {
	var req createAlbumRequest
	if err := decode(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	fields := map[string]string{}
	date, err := time.Parse(time.DateOnly, req.ReleaseDate)
	if err != nil {
		fields["releaseDate"] = "must be a date in YYYY-MM-DD format"
	}
	artists := parseIDs(req.ArtistIDs, "artistIds", fields)
	genres := parseIDs(req.GenreIDs, "genreIds", fields)
	if len(fields) > 0 {
		h.fail(w, r, invalidFields(fields))
		return
	}

	a, err := h.app.CreateAlbum(r.Context(), application.CreateAlbum{
		Title: req.Title, Type: domain.AlbumType(strings.ToUpper(req.AlbumType)), ReleaseDate: date,
		ArtistIDs: artists, GenreIDs: genres,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/albums/"+a.ID.String())
	httpserver.WriteJSON(w, http.StatusCreated, toAlbum(a))
}

func (h *Handler) getAlbum(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	a, err := h.app.GetAlbum(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toAlbum(a))
}

func (h *Handler) listAlbumTracks(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	tracks, err := h.app.ListAlbumTracks(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	// An album's track list is bounded, so it is returned in one page.
	httpserver.WriteJSON(w, http.StatusOK, listResponse[trackResponse]{Data: mapSlice(tracks, toTrack)})
}

func (h *Handler) createTrack(w http.ResponseWriter, r *http.Request) {
	var req createTrackRequest
	if err := decode(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	fields := map[string]string{}
	albumID, err := uuid.Parse(req.AlbumID)
	if err != nil {
		fields["albumId"] = "must be a UUID"
	}
	artists := parseIDs(req.ArtistIDs, "artistIds", fields)
	if len(fields) > 0 {
		h.fail(w, r, invalidFields(fields))
		return
	}

	t, err := h.app.CreateTrack(r.Context(), application.CreateTrack{
		AlbumID: albumID, ArtistIDs: artists, Title: req.Title,
		Duration: time.Duration(req.DurationMs) * time.Millisecond, TrackNumber: req.TrackNumber,
		DiscNumber: req.DiscNumber, Explicit: req.Explicit, ISRC: req.ISRC,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/tracks/"+t.ID.String())
	httpserver.WriteJSON(w, http.StatusCreated, toTrack(t))
}

func (h *Handler) getTrack(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	t, err := h.app.GetTrack(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toTrack(t))
}

func (h *Handler) updateTrack(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req updateTrackRequest
	if err := decode(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	changes := domain.TrackChanges{Title: req.Title, Explicit: req.Explicit}
	if req.Status != nil {
		s := domain.TrackStatus(strings.ToUpper(*req.Status))
		if !s.Valid() {
			h.fail(w, r, invalidFields(map[string]string{"status": "must be one of DRAFT, PROCESSING, READY, BLOCKED, DELETED"}))
			return
		}
		changes.Status = &s
	}
	t, err := h.app.UpdateTrack(r.Context(), application.UpdateTrack{ID: id, Changes: changes})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, toTrack(t))
}

func (h *Handler) listGenres(w http.ResponseWriter, r *http.Request) {
	genres, err := h.app.ListGenres(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, listResponse[genreResponse]{Data: mapSlice(genres, toGenre)})
}

// --- request helpers -------------------------------------------------------

func decode(w http.ResponseWriter, r *http.Request, dst any) error {
	if ct := r.Header.Get("Content-Type"); ct != "" && !strings.HasPrefix(ct, "application/json") {
		return &requestError{status: http.StatusUnsupportedMediaType, code: httpserver.CodeUnsupportedType, message: "Content-Type must be application/json"}
	}
	if err := httpserver.DecodeJSON(w, r, dst); err != nil {
		return badRequest(err.Error())
	}
	return nil
}

func pathID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return uuid.Nil, badRequest("path parameter id must be a UUID")
	}
	return id, nil
}

func parseIDs(raw []string, field string, fields map[string]string) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(raw))
	for _, s := range raw {
		id, err := uuid.Parse(s)
		if err != nil {
			fields[field] = "must contain UUIDs only"
			return nil
		}
		out = append(out, id)
	}
	return out
}

func pageParams(r *http.Request) (int, *ports.AlbumCursor, error) {
	q := r.URL.Query()
	limit := 0
	if s := q.Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > application.MaxPageSize {
			return 0, nil, invalidFields(map[string]string{"limit": "must be between 1 and " + strconv.Itoa(application.MaxPageSize)})
		}
		limit = n
	}
	var after *ports.AlbumCursor
	if s := q.Get("cursor"); s != "" {
		c, err := decodeCursor(s)
		if err != nil {
			return 0, nil, &requestError{status: http.StatusBadRequest, code: codeInvalidCursor, message: "cursor is malformed"}
		}
		after = &c
	}
	return limit, after, nil
}

// Cursors are opaque to clients (specs/api/pagination.md).
type cursorJSON struct {
	D string `json:"d"`
	I string `json:"i"`
}

func encodeCursor(c ports.AlbumCursor) string {
	b, _ := json.Marshal(cursorJSON{D: c.ReleaseDate.Format(time.DateOnly), I: c.ID.String()})
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(s string) (ports.AlbumCursor, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return ports.AlbumCursor{}, err
	}
	var c cursorJSON
	if err := json.Unmarshal(b, &c); err != nil {
		return ports.AlbumCursor{}, err
	}
	d, err := time.Parse(time.DateOnly, c.D)
	if err != nil {
		return ports.AlbumCursor{}, err
	}
	id, err := uuid.Parse(c.I)
	if err != nil {
		return ports.AlbumCursor{}, errors.New("bad cursor id")
	}
	return ports.AlbumCursor{ReleaseDate: d, ID: id}, nil
}
