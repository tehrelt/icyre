//go:build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	platformpg "github.com/tehrelt/icyre/libs/platform/postgres"
	"github.com/tehrelt/icyre/services/catalog/internal/domain"
	"github.com/tehrelt/icyre/services/catalog/internal/ports"
)

// newTestDB creates a throw-away database on the server behind
// CATALOG_TEST_DATABASE_DSN, applies migrations and drops it afterwards.
func newTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("CATALOG_TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("CATALOG_TEST_DATABASE_DSN is not set")
	}
	ctx := context.Background()

	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("catalog_it_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
		_ = admin.Close(context.Background())
	})

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := platformpg.Migrate(ctx, pool, Schema, Migrations(), log); err != nil {
		t.Fatal(err)
	}
	// Migrations are idempotent: a second run is a no-op.
	if err := platformpg.Migrate(ctx, pool, Schema, Migrations(), log); err != nil {
		t.Fatal(err)
	}
	return pool
}

var now = time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)

func newID() uuid.UUID { return uuid.Must(uuid.NewV7()) }

func TestCatalogRepositories(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	artists := NewArtistRepository(pool)
	albums := NewAlbumRepository(pool)
	tracks := NewTrackRepository(pool)
	genres := NewGenreRepository(pool)

	// Artists.
	nova, _ := domain.NewArtist(newID(), "Nova Hale", now)
	kai, _ := domain.NewArtist(newID(), "Kai Frost", now)
	for _, a := range []domain.Artist{nova, kai} {
		if err := artists.Create(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	got, err := artists.Get(ctx, nova.ID)
	if err != nil || got.Name != "Nova Hale" || !got.CreatedAt.Equal(now) {
		t.Fatalf("artist = %+v, err = %v", got, err)
	}
	if _, err := artists.Get(ctx, newID()); !errors.Is(err, domain.ErrArtistNotFound) {
		t.Fatalf("expected ErrArtistNotFound, got %v", err)
	}

	// Genres are seeded.
	gs, err := genres.List(ctx)
	if err != nil || len(gs) < 5 {
		t.Fatalf("genres = %v, err = %v", gs, err)
	}

	// Albums keep credit order and round-trip.
	album, _ := domain.NewAlbum(domain.NewAlbumParams{
		ID: newID(), Title: "Prism Hours", Type: domain.AlbumTypeAlbum,
		ReleaseDate: time.Date(2026, 3, 6, 0, 0, 0, 0, time.UTC),
		ArtistIDs:   []uuid.UUID{kai.ID, nova.ID}, GenreIDs: []uuid.UUID{gs[1].ID, gs[0].ID},
	}, now)
	if err := albums.Create(ctx, album); err != nil {
		t.Fatal(err)
	}
	gotAlbum, err := albums.Get(ctx, album.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotAlbum.ArtistIDs[0] != kai.ID || gotAlbum.GenreIDs[0] != gs[1].ID || !gotAlbum.ReleaseDate.Equal(album.ReleaseDate) {
		t.Fatalf("album round-trip mismatch: %+v", gotAlbum)
	}

	// FK constraint → ReferenceError; the failed transaction leaves nothing behind.
	bad, _ := domain.NewAlbum(domain.NewAlbumParams{ID: newID(), Title: "Ghost", Type: domain.AlbumTypeEP, ReleaseDate: now, ArtistIDs: []uuid.UUID{newID()}}, now)
	var ref *domain.ReferenceError
	if err := albums.Create(ctx, bad); !errors.As(err, &ref) || ref.Field != "artistIds" {
		t.Fatalf("expected artistIds reference error, got %v", err)
	}
	if _, err := albums.Get(ctx, bad.ID); !errors.Is(err, domain.ErrAlbumNotFound) {
		t.Fatal("failed album insert was not rolled back")
	}

	// Tracks.
	mk := func(n int, title string) domain.Track {
		tr, err := domain.NewTrack(domain.NewTrackParams{
			ID: newID(), AlbumID: album.ID, ArtistIDs: []uuid.UUID{nova.ID}, Title: title,
			Duration: 227 * time.Second, TrackNumber: n, ISRC: "USRC17607839",
		}, now)
		if err != nil {
			t.Fatal(err)
		}
		return tr
	}
	second, first := mk(2, "Glass Tides"), mk(1, "Frozen Choir")
	for _, tr := range []domain.Track{second, first} {
		if err := tracks.Create(ctx, tr); err != nil {
			t.Fatal(err)
		}
	}
	if err := tracks.Create(ctx, mk(2, "Duplicate")); !errors.Is(err, domain.ErrTrackPositionTaken) {
		t.Fatalf("expected ErrTrackPositionTaken, got %v", err)
	}

	list, err := tracks.ListByAlbum(ctx, album.ID)
	if err != nil || len(list) != 2 || list[0].ID != first.ID || list[1].Duration != 227*time.Second {
		t.Fatalf("list = %+v, err = %v", list, err)
	}

	// GetForUpdate holds the row lock until the transaction ends: a second
	// locker times out instead of reading a status about to change.
	tx := platformpg.Transactor{Pool: pool}
	err = tx.InTx(ctx, func(ctx context.Context) error {
		if got, err := tracks.GetForUpdate(ctx, second.ID); err != nil || got.ID != second.ID {
			return fmt.Errorf("GetForUpdate = %+v, %v", got, err)
		}
		// The failed statement aborts the other transaction; errLocked rolls
		// it back instead of committing.
		errLocked := errors.New("locked")
		err := platformpg.InTx(context.Background(), pool, func(other context.Context) error {
			if _, err := platformpg.Conn(other, pool).Exec(other, `SET LOCAL lock_timeout = '200ms'`); err != nil {
				return err
			}
			if _, err := tracks.GetForUpdate(other, second.ID); err != nil {
				return errLocked
			}
			return nil
		})
		if !errors.Is(err, errLocked) {
			return fmt.Errorf("second GetForUpdate must wait for the lock, got %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tracks.GetForUpdate(ctx, newID()); !errors.Is(err, domain.ErrTrackNotFound) {
		t.Fatalf("GetForUpdate of unknown track: %v", err)
	}

	// Update, then delete frees the position.
	deleted := domain.TrackStatusDeleted
	if _, err := first.Apply(domain.TrackChanges{Status: &deleted}, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := tracks.Update(ctx, first); err != nil {
		t.Fatal(err)
	}
	if got, _ := tracks.Get(ctx, first.ID); got.Status != domain.TrackStatusDeleted {
		t.Fatalf("status = %s", got.Status)
	}
	if byIDs, err := tracks.ListByIDs(ctx, []uuid.UUID{first.ID, second.ID, newID()}); err != nil || len(byIDs) != 1 || byIDs[0].ID != second.ID {
		t.Fatalf("ListByIDs must skip deleted and unknown: %+v, %v", byIDs, err)
	}
	if err := tracks.Create(ctx, mk(1, "Frozen Choir (new master)")); err != nil {
		t.Fatalf("deleted track must free its position: %v", err)
	}

	// DB CHECK constraints guard invariants even if the domain is bypassed.
	if _, err := pool.Exec(ctx, `UPDATE catalog.tracks SET status = 'LIVE' WHERE id = $1`, second.ID); err == nil {
		t.Fatal("expected check constraint violation for unknown status")
	}
}

func TestListByArtistKeysetPagination(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	artists := NewArtistRepository(pool)
	albums := NewAlbumRepository(pool)

	a, _ := domain.NewArtist(newID(), "Nova Hale", now)
	if err := artists.Create(ctx, a); err != nil {
		t.Fatal(err)
	}
	for year := 2019; year <= 2023; year++ {
		al, _ := domain.NewAlbum(domain.NewAlbumParams{ID: newID(), Title: fmt.Sprint(year), Type: domain.AlbumTypeAlbum,
			ReleaseDate: time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC), ArtistIDs: []uuid.UUID{a.ID}}, now)
		if err := albums.Create(ctx, al); err != nil {
			t.Fatal(err)
		}
	}

	page1, err := albums.ListByArtist(ctx, a.ID, nil, 2)
	if err != nil || len(page1) != 2 || page1[0].Title != "2023" {
		t.Fatalf("page1 = %+v, err = %v", page1, err)
	}
	last := page1[1]
	page2, err := albums.ListByArtist(ctx, a.ID, &ports.AlbumCursor{ReleaseDate: last.ReleaseDate, ID: last.ID}, 10)
	if err != nil || len(page2) != 3 || page2[0].Title != "2021" {
		t.Fatalf("page2 = %+v, err = %v", page2, err)
	}
	all, err := albums.List(ctx, nil, 3)
	if err != nil || len(all) != 3 || all[0].Title != "2023" || len(all[0].ArtistIDs) != 1 {
		t.Fatalf("list = %+v, err = %v", all, err)
	}

	found, err := artists.ListByIDs(ctx, []uuid.UUID{a.ID, newID()})
	if err != nil || len(found) != 1 || found[0].Name != "Nova Hale" {
		t.Fatalf("ListByIDs = %+v, err = %v", found, err)
	}
}
