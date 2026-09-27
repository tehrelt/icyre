// Package analytics writes playback events straight into ClickHouse inside
// the request (ANALYTICS_MODE=sync).
//
// It exists for the EPIC-038 experiment "sync analytics vs Kafka": the
// request pays for the Catalog lookup and the ClickHouse insert that the
// Analytics Worker does asynchronously in the default Kafka mode. Events
// written this way do not reach Kafka, so Listening History and
// recommendations do not see them.
package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/contracts/analytics"
	"github.com/tehrelt/icyre/libs/contracts/events/playbackv1"
	"github.com/tehrelt/icyre/services/playback/internal/domain"
)

// Inserter is the part of the platform ClickHouse client used here.
type Inserter interface {
	Insert(ctx context.Context, table string, rows []any, dedupToken string) error
}

// Publisher implements application.Publisher.
type Publisher struct {
	catalog string
	hc      *http.Client
	ch      Inserter
	newID   func() (uuid.UUID, error)
}

// New returns a Publisher resolving tracks in Catalog at catalogURL.
func New(catalogURL string, hc *http.Client, ch Inserter) *Publisher {
	return &Publisher{catalog: strings.TrimRight(catalogURL, "/"), hc: hc, ch: ch, newID: uuid.NewV7}
}

var types = map[domain.Kind]string{
	domain.Started:  playbackv1.TypeStarted,
	domain.Finished: playbackv1.TypeFinished,
	domain.Skipped:  playbackv1.TypeSkipped,
}

// timeLayout is how DateTime64(3) values are sent in JSONEachRow.
const timeLayout = "2006-01-02 15:04:05.000"

// row mirrors analytics.TablePlaybackEvents (as the Analytics Worker writes it).
type row struct {
	EventID    string   `json:"event_id"`
	EventType  string   `json:"event_type"`
	PlaybackID string   `json:"playback_id"`
	UserID     string   `json:"user_id"`
	TrackID    string   `json:"track_id"`
	AlbumID    string   `json:"album_id"`
	ArtistIDs  []string `json:"artist_ids"`
	Source     string   `json:"source"`
	DurationMs uint32   `json:"duration_ms"`
	ListenedMs uint32   `json:"listened_ms"`
	At         string   `json:"at"`
}

// Publish enriches the event from Catalog and inserts it.
func (p *Publisher) Publish(ctx context.Context, e domain.Event) error {
	t, err := p.track(ctx, e.TrackID)
	if err != nil {
		return err
	}
	id, err := p.newID()
	if err != nil {
		return fmt.Errorf("event id: %w", err)
	}
	kind, _, _ := strings.Cut(e.Source, ":")
	r := row{
		EventID: id.String(), EventType: types[e.Kind], PlaybackID: e.PlaybackID.String(),
		UserID: e.UserID.String(), TrackID: e.TrackID.String(), AlbumID: t.AlbumID, ArtistIDs: t.ArtistIDs,
		Source: kind, DurationMs: clampMs(e.DurationMs), ListenedMs: clampMs(e.ListenedMs), At: e.At.UTC().Format(timeLayout),
	}
	if r.AlbumID == "" {
		r.AlbumID = uuid.Nil.String()
	}
	if r.ArtistIDs == nil {
		r.ArtistIDs = []string{}
	}
	if err := p.ch.Insert(ctx, analytics.TablePlaybackEvents, []any{r}, id.String()); err != nil {
		return fmt.Errorf("insert playback event: %w", err)
	}
	return nil
}

type trackDTO struct {
	AlbumID   string   `json:"albumId"`
	ArtistIDs []string `json:"artistIds"`
}

// track resolves the album and artists; a track unknown to Catalog yields
// an empty result, and the event is stored without them (as the Analytics
// Worker does for deleted tracks).
func (p *Publisher) track(ctx context.Context, id uuid.UUID) (trackDTO, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.catalog+"/api/v1/tracks/"+id.String(), nil)
	if err != nil {
		return trackDTO{}, err
	}
	res, err := p.hc.Do(req)
	if err != nil {
		return trackDTO{}, fmt.Errorf("catalog: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	switch {
	case res.StatusCode == http.StatusNotFound:
		return trackDTO{}, nil
	case res.StatusCode != http.StatusOK:
		return trackDTO{}, fmt.Errorf("catalog: status %d", res.StatusCode)
	}
	var t trackDTO
	if err := json.NewDecoder(res.Body).Decode(&t); err != nil {
		return trackDTO{}, fmt.Errorf("catalog: decode track: %w", err)
	}
	return t, nil
}

func clampMs(ms int64) uint32 {
	return uint32(min(max(ms, 0), math.MaxUint32))
}
