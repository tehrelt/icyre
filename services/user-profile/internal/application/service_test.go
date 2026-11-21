package application

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/platform/objectstore"
	"github.com/tehrelt/icyre/services/user-profile/internal/domain"
)

type memRepo struct {
	profiles map[uuid.UUID]domain.Profile
	uploads  map[uuid.UUID]domain.AvatarUpload
}

func newMemRepo() *memRepo {
	return &memRepo{
		profiles: map[uuid.UUID]domain.Profile{},
		uploads:  map[uuid.UUID]domain.AvatarUpload{},
	}
}

func (m *memRepo) taken(p domain.Profile) bool {
	for id, x := range m.profiles {
		if id != p.UserID && x.Username == p.Username {
			return true
		}
	}
	return false
}

func (m *memRepo) CreateIfAbsent(_ context.Context, p domain.Profile) (bool, error) {
	if _, ok := m.profiles[p.UserID]; ok {
		return false, nil
	}
	if m.taken(p) {
		return false, domain.ErrUsernameTaken
	}
	m.profiles[p.UserID] = p
	return true, nil
}

func (m *memRepo) Get(_ context.Context, id uuid.UUID) (domain.Profile, error) {
	p, ok := m.profiles[id]
	if !ok {
		return domain.Profile{}, domain.ErrProfileNotFound
	}
	return p, nil
}

func (m *memRepo) GetForUpdate(ctx context.Context, id uuid.UUID) (domain.Profile, error) {
	return m.Get(ctx, id)
}

func (m *memRepo) Update(_ context.Context, p domain.Profile) error {
	cur, ok := m.profiles[p.UserID]
	if !ok {
		return domain.ErrProfileNotFound
	}
	if m.taken(p) {
		return domain.ErrUsernameTaken
	}
	p.AvatarKey = cur.AvatarKey
	m.profiles[p.UserID] = p
	return nil
}

func (m *memRepo) SetAvatarKey(_ context.Context, userID uuid.UUID, avatarKey string, updatedAt time.Time) error {
	p, ok := m.profiles[userID]
	if !ok {
		return domain.ErrProfileNotFound
	}
	p.AvatarKey = avatarKey
	p.UpdatedAt = updatedAt
	m.profiles[userID] = p
	return nil
}

func (m *memRepo) CreateAvatarUpload(_ context.Context, u domain.AvatarUpload) error {
	m.uploads[u.ID] = u
	return nil
}

func (m *memRepo) GetAvatarUpload(_ context.Context, id uuid.UUID) (domain.AvatarUpload, error) {
	u, ok := m.uploads[id]
	if !ok {
		return domain.AvatarUpload{}, domain.ErrAvatarUploadNotFound
	}
	return u, nil
}

func (m *memRepo) SettleAvatarUpload(_ context.Context, u domain.AvatarUpload) (bool, error) {
	cur, ok := m.uploads[u.ID]
	if !ok || cur.Status != domain.AvatarStatusPending {
		return false, nil
	}
	m.uploads[u.ID] = u
	return true, nil
}

type memObject struct {
	body []byte
	ct   string
	sum  string
}

type memStore struct {
	objs       map[string]memObject
	presignErr error
	lastDLOpts objectstore.DownloadOptions
}

func newMemStore() *memStore {
	return &memStore{objs: map[string]memObject{}}
}

func (m *memStore) PresignUpload(_ context.Context, key string, ttl time.Duration, o objectstore.UploadOptions) (objectstore.SignedURL, error) {
	return objectstore.SignedURL{
		Method:    "PUT",
		URL:       "http://minio/" + key,
		Headers:   map[string]string{"Content-Type": o.ContentType},
		ExpiresAt: time.Now().Add(ttl),
	}, nil
}

func (m *memStore) PresignDownload(_ context.Context, key string, ttl time.Duration, o objectstore.DownloadOptions) (objectstore.SignedURL, error) {
	m.lastDLOpts = o
	if m.presignErr != nil {
		return objectstore.SignedURL{}, m.presignErr
	}
	return objectstore.SignedURL{
		Method:    "GET",
		URL:       "http://minio/" + key + "?signed=1",
		ExpiresAt: time.Now().Add(ttl),
	}, nil
}

func (m *memStore) Stat(_ context.Context, key string) (objectstore.ObjectInfo, error) {
	o, ok := m.objs[key]
	if !ok {
		return objectstore.ObjectInfo{}, objectstore.ErrNotFound
	}
	return objectstore.ObjectInfo{Key: key, Size: int64(len(o.body)), ContentType: o.ct, SHA256: o.sum}, nil
}

func (m *memStore) ReadHead(_ context.Context, key string, n int64) ([]byte, error) {
	o, ok := m.objs[key]
	if !ok {
		return nil, objectstore.ErrNotFound
	}
	return o.body[:min(int(n), len(o.body))], nil
}

func (m *memStore) Remove(_ context.Context, key string) error {
	delete(m.objs, key)
	return nil
}

func (m *memStore) put(key, ct string, body []byte) {
	sum := sha256.Sum256(body)
	m.objs[key] = memObject{body: body, ct: ct, sum: base64.StdEncoding.EncodeToString(sum[:])}
}

type pubRecorder struct {
	n             int
	avatarChanges []string
}

func (p *pubRecorder) ProfileUpdated(context.Context, domain.Profile) error { p.n++; return nil }
func (p *pubRecorder) AvatarChanged(_ context.Context, _ uuid.UUID, key string, _ time.Time) error {
	p.avatarChanges = append(p.avatarChanges, key)
	return nil
}

func newSvc() (*Service, *memRepo, *pubRecorder) {
	repo, pub := newMemRepo(), &pubRecorder{}
	return New(repo, pub, nil, slog.New(slog.NewTextHandler(io.Discard, nil))), repo, pub
}

func newAvatarSvc() (*Service, *memRepo, *memStore, *pubRecorder, time.Time) {
	repo, store, pub := newMemRepo(), newMemStore(), &pubRecorder{}
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	svc := New(repo, pub, nil, slog.New(slog.NewTextHandler(io.Discard, nil))).
		WithAvatarStorage(store, AvatarLimits{MaxSize: 1 << 20, UploadURLTTL: 15 * time.Minute, DownloadURLTTL: 10 * time.Minute})
	svc.now = func() time.Time { return now }
	return svc, repo, store, pub, now
}

func avatarReq(ct string, body []byte) domain.AvatarUploadRequest {
	sum := sha256.Sum256(body)
	return domain.AvatarUploadRequest{
		ContentType: ct,
		SizeBytes:   int64(len(body)),
		SHA256Hex:   hex.EncodeToString(sum[:]),
	}
}

var (
	pngBytes  = append([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, make([]byte, 32)...)
	webpBytes = append([]byte("RIFF\x24\x00\x00\x00WEBP"), make([]byte, 32)...)
)

func TestCreateForNewUserIsIdempotent(t *testing.T) {
	svc, repo, pub := newSvc()
	id := uuid.New()
	for i := 0; i < 3; i++ { // redelivered event
		if err := svc.CreateForNewUser(context.Background(), id, "rin@example.com"); err != nil {
			t.Fatal(err)
		}
	}
	if len(repo.profiles) != 1 || pub.n != 1 {
		t.Fatalf("profiles=%d events=%d", len(repo.profiles), pub.n)
	}
}

func TestUsernameCollisionsGetSuffixes(t *testing.T) {
	svc, repo, _ := newSvc()
	ctx := context.Background()
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	_ = svc.CreateForNewUser(ctx, a, "rin@one.com")
	_ = svc.CreateForNewUser(ctx, b, "rin@two.com")
	_ = svc.CreateForNewUser(ctx, c, "rin@three.com")
	if repo.profiles[a].Username != "rin" || repo.profiles[b].Username != "rin2" || repo.profiles[c].Username != "rin3" {
		t.Fatalf("usernames %s %s %s", repo.profiles[a].Username, repo.profiles[b].Username, repo.profiles[c].Username)
	}
}

// countingTx counts units of work and the ones that failed (rolled back).
type countingTx struct{ runs, rolledBack int }

func (c *countingTx) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	c.runs++
	err := fn(ctx)
	if err != nil {
		c.rolledBack++
	}
	return err
}

type failingPub struct{}

func (failingPub) ProfileUpdated(context.Context, domain.Profile) error {
	return errors.New("outbox insert failed")
}
func (failingPub) AvatarChanged(context.Context, uuid.UUID, string, time.Time) error {
	return errors.New("outbox insert failed")
}

func TestEachUsernameAttemptIsItsOwnTransaction(t *testing.T) {
	repo, tx := newMemRepo(), &countingTx{}
	svc := New(repo, &pubRecorder{}, tx, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()
	_ = svc.CreateForNewUser(ctx, uuid.New(), "rin@example.com")
	// The clash rolls its transaction back; the retry runs in a fresh one.
	if err := svc.CreateForNewUser(ctx, uuid.New(), "rin@example.org"); err != nil || tx.runs != 3 || tx.rolledBack != 1 {
		t.Fatalf("err %v, tx %+v", err, tx)
	}

	tx = &countingTx{}
	svc = New(newMemRepo(), failingPub{}, tx, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := svc.CreateForNewUser(ctx, uuid.New(), "kai@example.com"); err == nil || tx.rolledBack != 1 {
		t.Fatalf("a failed event write must fail (and retry via Kafka): %v, %+v", err, tx)
	}
}

func TestUpdate(t *testing.T) {
	svc, _, pub := newSvc()
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	_ = svc.CreateForNewUser(ctx, a, "rin@example.com")
	_ = svc.CreateForNewUser(ctx, b, "kai@example.com")

	name := "Rin Aoki"
	p, err := svc.Update(ctx, a, domain.Changes{DisplayName: &name})
	if err != nil || p.DisplayName != "Rin Aoki" || pub.n != 3 {
		t.Fatalf("p=%+v err=%v events=%d", p, err, pub.n)
	}
	taken := "kai"
	if _, err := svc.Update(ctx, a, domain.Changes{Username: &taken}); !errors.Is(err, domain.ErrUsernameTaken) {
		t.Fatalf("expected ErrUsernameTaken, got %v", err)
	}
	if _, err := svc.Update(ctx, uuid.New(), domain.Changes{DisplayName: &name}); !errors.Is(err, domain.ErrProfileNotFound) {
		t.Fatalf("missing profile: %v", err)
	}
}

func TestAvatarUploadFlowAndReplaceAndDelete(t *testing.T) {
	svcNoStore, _, _ := newSvc()
	if _, err := svcNoStore.CreateAvatarUpload(context.Background(), uuid.New(), avatarReq("image/png", pngBytes)); !errors.Is(err, domain.ErrStorageUnavailable) {
		t.Fatalf("expected ErrStorageUnavailable, got %v", err)
	}
	if _, err := svcNoStore.CompleteAvatarUpload(context.Background(), uuid.New(), uuid.New()); !errors.Is(err, domain.ErrStorageUnavailable) {
		t.Fatalf("expected ErrStorageUnavailable, got %v", err)
	}

	svc, _, store, pub, _ := newAvatarSvc()
	ctx := context.Background()
	userID := uuid.New()
	_ = svc.CreateForNewUser(ctx, userID, "rin@example.com")

	if _, err := svc.CreateAvatarUpload(ctx, uuid.New(), avatarReq("image/png", pngBytes)); !errors.Is(err, domain.ErrProfileNotFound) {
		t.Fatalf("missing profile: %v", err)
	}

	c1, err := svc.CreateAvatarUpload(ctx, userID, avatarReq("image/png", pngBytes))
	if err != nil || c1.URL.Method != "PUT" || c1.Upload.Status != domain.AvatarStatusPending {
		t.Fatalf("create1: %+v %v", c1, err)
	}
	if _, err := svc.CompleteAvatarUpload(ctx, userID, c1.Upload.ID); !errors.Is(err, domain.ErrAvatarNotUploaded) {
		t.Fatalf("before upload: %v", err)
	}
	if _, err := svc.CompleteAvatarUpload(ctx, uuid.New(), c1.Upload.ID); !errors.Is(err, domain.ErrAvatarUploadNotFound) {
		t.Fatalf("other user: %v", err)
	}

	store.put(c1.Upload.Key, "image/png", pngBytes)
	for range 2 { // idempotent
		p, err := svc.CompleteAvatarUpload(ctx, userID, c1.Upload.ID)
		if err != nil || p.AvatarKey != c1.Upload.Key {
			t.Fatalf("complete1: %+v %v", p, err)
		}
	}
	if pub.n != 2 || len(pub.avatarChanges) != 1 || pub.avatarChanges[0] != c1.Upload.Key {
		t.Fatalf("events n=%d avatarChanges=%v", pub.n, pub.avatarChanges)
	}

	// Replace avatar: old object is removed best-effort.
	c2, err := svc.CreateAvatarUpload(ctx, userID, avatarReq("image/webp", webpBytes))
	if err != nil {
		t.Fatal(err)
	}
	store.put(c2.Upload.Key, "image/webp", webpBytes)
	p, err := svc.CompleteAvatarUpload(ctx, userID, c2.Upload.ID)
	if err != nil || p.AvatarKey != c2.Upload.Key {
		t.Fatalf("complete2: %+v %v", p, err)
	}
	if _, oldExists := store.objs[c1.Upload.Key]; oldExists {
		t.Fatal("previous avatar object should have been removed")
	}

	// Delete avatar clears key, publishes events, removes object; second delete is no-op.
	p, err = svc.DeleteAvatar(ctx, userID)
	if err != nil || p.AvatarKey != "" {
		t.Fatalf("delete: %+v %v", p, err)
	}
	if _, exists := store.objs[c2.Upload.Key]; exists {
		t.Fatal("deleted avatar object should have been removed")
	}
	if len(pub.avatarChanges) != 3 || pub.avatarChanges[2] != "" {
		t.Fatalf("avatarChanges=%v", pub.avatarChanges)
	}
	if _, err := svc.DeleteAvatar(ctx, userID); err != nil || len(pub.avatarChanges) != 3 {
		t.Fatalf("second delete should be no-op: %v %v", err, pub.avatarChanges)
	}
}

func TestAvatarRejectionsAndExpiry(t *testing.T) {
	cases := map[string]struct {
		ct   string
		body []byte
	}{
		domain.ReasonSizeMismatch:        {"image/png", pngBytes[:10]},
		domain.ReasonContentTypeMismatch: {"image/jpeg", pngBytes},
		domain.ReasonUnrecognizedFormat:  {"image/png", append([]byte("<html>"), pngBytes[6:]...)},
	}
	for want, tc := range cases {
		svc, _, store, pub, _ := newAvatarSvc()
		ctx := context.Background()
		userID := uuid.New()
		_ = svc.CreateForNewUser(ctx, userID, "rin@example.com")
		req := avatarReq("image/png", pngBytes)
		if want == domain.ReasonUnrecognizedFormat {
			req = avatarReq("image/png", tc.body)
		}
		cr, err := svc.CreateAvatarUpload(ctx, userID, req)
		if err != nil {
			t.Fatal(err)
		}
		store.put(cr.Upload.Key, tc.ct, tc.body)
		_, err = svc.CompleteAvatarUpload(ctx, userID, cr.Upload.ID)
		var re *domain.AvatarRejectedError
		if !errors.As(err, &re) || re.Reason != want {
			t.Fatalf("%s: got %v", want, err)
		}
		if _, left := store.objs[cr.Upload.Key]; left || len(pub.avatarChanges) != 0 {
			t.Fatalf("%s: left=%v changes=%v", want, left, pub.avatarChanges)
		}
		// Repeating complete on failed upload returns the same rejection.
		if _, err := svc.CompleteAvatarUpload(ctx, userID, cr.Upload.ID); !errors.As(err, &re) || re.Reason != want {
			t.Fatalf("%s repeat: got %v", want, err)
		}
	}

	// Checksum mismatch and expiry.
	svc, _, store, _, now := newAvatarSvc()
	ctx := context.Background()
	userID := uuid.New()
	_ = svc.CreateForNewUser(ctx, userID, "rin@example.com")
	cr, _ := svc.CreateAvatarUpload(ctx, userID, avatarReq("image/png", pngBytes))
	store.objs[cr.Upload.Key] = memObject{body: pngBytes, ct: "image/png", sum: "bogus"}
	var re *domain.AvatarRejectedError
	if _, err := svc.CompleteAvatarUpload(ctx, userID, cr.Upload.ID); !errors.As(err, &re) || re.Reason != domain.ReasonChecksumMismatch {
		t.Fatalf("checksum: %v", err)
	}

	cr2, _ := svc.CreateAvatarUpload(ctx, userID, avatarReq("image/png", pngBytes))
	svc.now = func() time.Time { return now.Add(time.Hour) }
	if _, err := svc.CompleteAvatarUpload(ctx, userID, cr2.Upload.ID); !errors.As(err, &re) || re.Reason != domain.ReasonExpired {
		t.Fatalf("expired: %v", err)
	}
}

func TestAvatarURL(t *testing.T) {
	svcNoStore, _, _ := newSvc()
	if u, err := svcNoStore.AvatarURL(context.Background(), "users/1/avatars/2.png"); err != nil || u != nil {
		t.Fatalf("no store: %v %v", u, err)
	}

	svc, _, store, _, _ := newAvatarSvc()
	if u, err := svc.AvatarURL(context.Background(), ""); err != nil || u != nil {
		t.Fatalf("empty key: %v %v", u, err)
	}
	u, err := svc.AvatarURL(context.Background(), "users/1/avatars/2.png")
	if err != nil || u == nil || *u == "" || store.lastDLOpts.CacheControl != "private, max-age=600" {
		t.Fatalf("url=%v err=%v opts=%+v", u, err, store.lastDLOpts)
	}

	store.presignErr = errors.New("minio down")
	if u, err := svc.AvatarURL(context.Background(), "users/1/avatars/2.png"); err != nil || u != nil {
		t.Fatalf("presign error must degrade to nil: %v %v", u, err)
	}
}
