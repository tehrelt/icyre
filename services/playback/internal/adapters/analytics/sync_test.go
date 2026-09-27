package analytics

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/contracts/analytics"
	"github.com/tehrelt/icyre/services/playback/internal/domain"
)

type fakeCH struct {
	table string
	rows  []any
	token string
	err   error
}

func (f *fakeCH) Insert(_ context.Context, table string, rows []any, token string) error {
	f.table, f.rows, f.token = table, rows, token
	return f.err
}

func catalogServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func event() domain.Event {
	return domain.Event{
		Kind: domain.Finished, PlaybackID: uuid.New(), UserID: uuid.New(), TrackID: uuid.New(),
		Source: "album:a1", DurationMs: 180_000, ListenedMs: 175_000,
		At: time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC),
	}
}

func TestPublishEnrichesAndInserts(t *testing.T) {
	album, artist := uuid.NewString(), uuid.NewString()
	srv := catalogServer(t, http.StatusOK, `{"albumId":"`+album+`","artistIds":["`+artist+`"]}`)
	ch := &fakeCH{}
	e := event()
	if err := New(srv.URL, srv.Client(), ch).Publish(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	if ch.table != analytics.TablePlaybackEvents || len(ch.rows) != 1 || ch.token == "" {
		t.Fatalf("insert = %s %d rows token %q", ch.table, len(ch.rows), ch.token)
	}
	r := ch.rows[0].(row)
	if r.EventType != "playback.finished" || r.AlbumID != album || len(r.ArtistIDs) != 1 || r.ArtistIDs[0] != artist ||
		r.Source != "album" || r.ListenedMs != 175_000 || r.At != "2026-09-27 10:00:00.000" || r.EventID != ch.token {
		t.Fatalf("row = %+v", r)
	}
}

func TestPublishUnknownTrackStoresWithoutArtists(t *testing.T) {
	srv := catalogServer(t, http.StatusNotFound, `{}`)
	ch := &fakeCH{}
	if err := New(srv.URL, srv.Client(), ch).Publish(context.Background(), event()); err != nil {
		t.Fatal(err)
	}
	r := ch.rows[0].(row)
	if r.AlbumID != uuid.Nil.String() || r.ArtistIDs == nil || len(r.ArtistIDs) != 0 {
		t.Fatalf("row = %+v", r)
	}
}

func TestPublishFailures(t *testing.T) {
	down := catalogServer(t, http.StatusServiceUnavailable, `{}`)
	if err := New(down.URL, down.Client(), &fakeCH{}).Publish(context.Background(), event()); err == nil {
		t.Fatal("catalog 503: want error")
	}
	ok := catalogServer(t, http.StatusOK, `{"albumId":"","artistIds":[]}`)
	if err := New(ok.URL, ok.Client(), &fakeCH{err: errors.New("boom")}).Publish(context.Background(), event()); err == nil {
		t.Fatal("insert failure: want error")
	}
}
