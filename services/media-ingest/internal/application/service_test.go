package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/platform/objectstore"
	"github.com/tehrelt/icyre/services/media-ingest/internal/domain"
)

type memRepo struct{ ups map[uuid.UUID]domain.Upload }

func (m *memRepo) Create(_ context.Context, u domain.Upload) error { m.ups[u.ID] = u; return nil }
func (m *memRepo) Get(_ context.Context, id uuid.UUID) (domain.Upload, error) {
	u, ok := m.ups[id]
	if !ok {
		return u, domain.ErrNotFound
	}
	return u, nil
}
func (m *memRepo) Settle(_ context.Context, u domain.Upload) (bool, error) {
	if m.ups[u.ID].Status != domain.StatusPending {
		return false, nil
	}
	m.ups[u.ID] = u
	return true, nil
}

type fakeCatalog struct{}

func (fakeCatalog) TrackExists(_ context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return domain.ErrTrackNotFound
	}
	return nil
}

type object struct {
	body []byte
	ct   string
	sum  string
}

type memStore struct{ objs map[string]object }

func (m *memStore) PresignUpload(_ context.Context, key string, ttl time.Duration, o objectstore.UploadOptions) (objectstore.SignedURL, error) {
	return objectstore.SignedURL{Method: "PUT", URL: "http://minio/" + key, Headers: map[string]string{"Content-Type": o.ContentType}}, nil
}
func (m *memStore) Stat(_ context.Context, key string) (objectstore.ObjectInfo, error) {
	o, ok := m.objs[key]
	if !ok {
		return objectstore.ObjectInfo{}, objectstore.ErrNotFound
	}
	return objectstore.ObjectInfo{Key: key, Size: int64(len(o.body)), ContentType: o.ct, SHA256: o.sum}, nil
}
func (m *memStore) ReadHead(_ context.Context, key string, n int64) ([]byte, error) {
	b := m.objs[key].body
	return b[:min(int(n), len(b))], nil
}
func (m *memStore) Remove(_ context.Context, key string) error { delete(m.objs, key); return nil }

// put stores body as a client upload would (the store computes the checksum).
func (m *memStore) put(key, ct string, body []byte) {
	sum := sha256.Sum256(body)
	m.objs[key] = object{body: body, ct: ct, sum: base64.StdEncoding.EncodeToString(sum[:])}
}

type recPub struct{ uploaded, failed []domain.Upload }

func (p *recPub) Uploaded(_ context.Context, u domain.Upload) error {
	p.uploaded = append(p.uploaded, u)
	return nil
}
func (p *recPub) Failed(_ context.Context, u domain.Upload) error {
	p.failed = append(p.failed, u)
	return nil
}

type fixture struct {
	svc   *Service
	repo  *memRepo
	store *memStore
	pub   *recPub
	now   time.Time
}

func newFixture() *fixture {
	f := &fixture{repo: &memRepo{ups: map[uuid.UUID]domain.Upload{}}, store: &memStore{objs: map[string]object{}}, pub: &recPub{}, now: time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)}
	f.svc = New(f.repo, fakeCatalog{}, f.store, f.pub, nil, Limits{MaxSize: 1 << 20, URLTTL: 15 * time.Minute}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	f.svc.now = func() time.Time { return f.now }
	return f
}

var flac = append([]byte("fLaC\x00\x00\x00\x22"), make([]byte, 100)...)

func request(body []byte) domain.Request {
	sum := sha256.Sum256(body)
	return domain.Request{TrackID: uuid.New(), ContentType: "audio/flac", SizeBytes: int64(len(body)), SHA256Hex: hex.EncodeToString(sum[:])}
}

func TestUploadCompletes(t *testing.T) {
	f := newFixture()
	artist := Actor{ID: uuid.New(), Artist: true}
	c, err := f.svc.Create(context.Background(), artist, request(flac))
	if err != nil {
		t.Fatal(err)
	}
	if c.Upload.Status != domain.StatusPending || c.URL.Method != "PUT" || c.Upload.Key != "tracks/"+c.Upload.TrackID.String()+"/original/"+c.Upload.ID.String()+".flac" {
		t.Fatalf("%+v", c)
	}
	if _, err := f.svc.Complete(context.Background(), artist, c.Upload.ID); !errors.Is(err, domain.ErrNotUploaded) {
		t.Fatalf("complete before upload: %v", err)
	}
	f.store.put(c.Upload.Key, "audio/flac", flac)
	for range 2 { // idempotent: one event
		u, err := f.svc.Complete(context.Background(), artist, c.Upload.ID)
		if err != nil || u.Status != domain.StatusCompleted {
			t.Fatal(u, err)
		}
	}
	if len(f.pub.uploaded) != 1 || len(f.pub.failed) != 0 {
		t.Fatalf("events %+v", f.pub)
	}
	// Other users do not see the session; admins do.
	if _, err := f.svc.Get(context.Background(), Actor{ID: uuid.New(), Artist: true}, c.Upload.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := f.svc.Get(context.Background(), Actor{ID: uuid.New(), Admin: true}, c.Upload.ID); err != nil {
		t.Fatal(err)
	}
}

func TestCreateRules(t *testing.T) {
	f := newFixture()
	if _, err := f.svc.Create(context.Background(), Actor{ID: uuid.New()}, request(flac)); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("listener: %v", err)
	}
	admin := Actor{ID: uuid.New(), Admin: true}
	req := request(flac)
	req.TrackID = uuid.Nil
	var ve *domain.ValidationError
	if _, err := f.svc.Create(context.Background(), admin, req); !errors.As(err, &ve) {
		t.Fatalf("no track: %v", err)
	}
	req = request(make([]byte, 2<<20))
	if _, err := f.svc.Create(context.Background(), admin, req); !errors.As(err, &ve) || ve.Fields["sizeBytes"] == "" {
		t.Fatalf("too large: %v", err)
	}
}

func TestRejections(t *testing.T) {
	cases := map[string]struct {
		ct   string
		body []byte
	}{
		"SIZE_MISMATCH":         {"audio/flac", flac[:50]},
		"CONTENT_TYPE_MISMATCH": {"audio/mpeg", flac},
		"UNRECOGNIZED_FORMAT":   {"audio/flac", append([]byte("<html>"), flac[6:]...)},
	}
	for want, c := range cases {
		f := newFixture()
		artist := Actor{ID: uuid.New(), Artist: true}
		req := request(flac)
		if want == "UNRECOGNIZED_FORMAT" {
			req = request(c.body)
		}
		cr, err := f.svc.Create(context.Background(), artist, req)
		if err != nil {
			t.Fatal(err)
		}
		f.store.put(cr.Upload.Key, c.ct, c.body)
		u, err := f.svc.Complete(context.Background(), artist, cr.Upload.ID)
		var re *domain.RejectedError
		if !errors.As(err, &re) || re.Reason != want || u.Status != domain.StatusFailed {
			t.Errorf("%s: %+v %v", want, u, err)
		}
		if _, left := f.store.objs[cr.Upload.Key]; left || len(f.pub.failed) != 1 {
			t.Errorf("%s: object kept=%v events=%d", want, left, len(f.pub.failed))
		}
	}
}

func TestChecksumAndExpiry(t *testing.T) {
	f := newFixture()
	artist := Actor{ID: uuid.New(), Artist: true}
	cr, _ := f.svc.Create(context.Background(), artist, request(flac))
	f.store.objs[cr.Upload.Key] = object{body: flac, ct: "audio/flac", sum: "bogus"}
	if _, err := f.svc.Complete(context.Background(), artist, cr.Upload.ID); err == nil || err.Error() != "upload rejected: CHECKSUM_MISMATCH" {
		t.Fatal(err)
	}

	cr, _ = f.svc.Create(context.Background(), artist, request(flac))
	f.now = f.now.Add(time.Hour)
	var re *domain.RejectedError
	if _, err := f.svc.Complete(context.Background(), artist, cr.Upload.ID); !errors.As(err, &re) || re.Reason != "EXPIRED" {
		t.Fatal(err)
	}
	if len(f.pub.failed) != 2 {
		t.Fatal(len(f.pub.failed))
	}
}

func TestAuditLoggingAndNilLoggerDefault(t *testing.T) {
	nilSvc := New(&memRepo{ups: map[uuid.UUID]domain.Upload{}}, fakeCatalog{}, &memStore{objs: map[string]object{}}, &recPub{}, nil, Limits{MaxSize: 1 << 20, URLTTL: 15 * time.Minute}, nil)
	if nilSvc.log == nil {
		t.Fatal("expected nil logger to default to slog.Default()")
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	repo := &memRepo{ups: map[uuid.UUID]domain.Upload{}}
	store := &memStore{objs: map[string]object{}}
	pub := &recPub{}
	svc := New(repo, fakeCatalog{}, store, pub, nil, Limits{MaxSize: 1 << 20, URLTTL: 15 * time.Minute}, logger)
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	ctx := context.Background()
	artist := Actor{ID: uuid.New(), Artist: true}

	c1, err := svc.Create(ctx, artist, request(flac))
	if err != nil {
		t.Fatal(err)
	}
	store.put(c1.Upload.Key, "audio/flac", flac)
	if _, err := svc.Complete(ctx, artist, c1.Upload.ID); err != nil {
		t.Fatal(err)
	}

	c2, err := svc.Create(ctx, artist, request(flac))
	if err != nil {
		t.Fatal(err)
	}
	store.put(c2.Upload.Key, "audio/mpeg", flac)
	if _, err := svc.Complete(ctx, artist, c2.Upload.ID); err == nil {
		t.Fatal("expected rejection")
	}

	logs := buf.String()
	for _, want := range []string{
		`"audit":"media_ingest"`,
		`"event":"upload_created"`,
		`"event":"upload_completed"`,
		`"event":"upload_rejected"`,
		`"upload_id":"` + c1.Upload.ID.String() + `"`,
		`"track_id":"` + c1.Upload.TrackID.String() + `"`,
		`"uploader_id":"` + artist.ID.String() + `"`,
		`"reason":"CONTENT_TYPE_MISMATCH"`,
	} {
		if !strings.Contains(logs, want) {
			t.Fatalf("missing %s in logs:\n%s", want, logs)
		}
	}
	if strings.Contains(logs, c1.URL.URL) || strings.Contains(logs, c2.URL.URL) {
		t.Fatalf("presigned URL leaked in logs:\n%s", logs)
	}
}

func TestAuditLoggingAdminCompletesOtherUsersUpload(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	repo := &memRepo{ups: map[uuid.UUID]domain.Upload{}}
	store := &memStore{objs: map[string]object{}}
	pub := &recPub{}
	svc := New(repo, fakeCatalog{}, store, pub, nil, Limits{MaxSize: 1 << 20, URLTTL: 15 * time.Minute}, logger)
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	ctx := context.Background()
	artist := Actor{ID: uuid.New(), Artist: true}
	admin := Actor{ID: uuid.New(), Admin: true}

	c1, err := svc.Create(ctx, artist, request(flac))
	if err != nil {
		t.Fatal(err)
	}
	store.put(c1.Upload.Key, "audio/flac", flac)
	if _, err := svc.Complete(ctx, admin, c1.Upload.ID); err != nil {
		t.Fatal(err)
	}

	c2, err := svc.Create(ctx, artist, request(flac))
	if err != nil {
		t.Fatal(err)
	}
	store.put(c2.Upload.Key, "audio/mpeg", flac)
	if _, err := svc.Complete(ctx, admin, c2.Upload.ID); err == nil {
		t.Fatal("expected rejection")
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 audit log lines, got %d:\n%s", len(lines), buf.String())
	}
	for _, idx := range []int{1, 3} {
		line := lines[idx]
		if !strings.Contains(line, `"uploader_id":"`+artist.ID.String()+`"`) {
			t.Fatalf("expected uploader_id=%s in %s", artist.ID, line)
		}
		if !strings.Contains(line, `"actor_id":"`+admin.ID.String()+`"`) {
			t.Fatalf("expected actor_id=%s in %s", admin.ID, line)
		}
	}
	if !strings.Contains(lines[1], `"event":"upload_completed"`) {
		t.Fatalf("expected upload_completed in %s", lines[1])
	}
	if !strings.Contains(lines[3], `"event":"upload_rejected"`) {
		t.Fatalf("expected upload_rejected in %s", lines[3])
	}
}
