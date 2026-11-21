// Package application implements User Profile use cases.
package application

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/contracts/media"
	"github.com/tehrelt/icyre/libs/platform/objectstore"
	"github.com/tehrelt/icyre/services/user-profile/internal/domain"
)

// Repository persists profiles and avatar upload sessions.
//
// CreateIfAbsent inserts unless a profile for the user exists (then created
// is false) and returns domain.ErrUsernameTaken on a username clash.
// Update returns domain.ErrUsernameTaken the same way.
type Repository interface {
	CreateIfAbsent(ctx context.Context, p domain.Profile) (created bool, err error)
	Get(ctx context.Context, userID uuid.UUID) (domain.Profile, error)
	GetForUpdate(ctx context.Context, userID uuid.UUID) (domain.Profile, error)
	Update(ctx context.Context, p domain.Profile) error
	SetAvatarKey(ctx context.Context, userID uuid.UUID, avatarKey string, updatedAt time.Time) error
	CreateAvatarUpload(ctx context.Context, u domain.AvatarUpload) error
	GetAvatarUpload(ctx context.Context, id uuid.UUID) (domain.AvatarUpload, error)
	SettleAvatarUpload(ctx context.Context, u domain.AvatarUpload) (bool, error)
}

// Publisher announces profile changes.
type Publisher interface {
	ProfileUpdated(ctx context.Context, p domain.Profile) error
	AvatarChanged(ctx context.Context, userID uuid.UUID, avatarKey string, at time.Time) error
}

// Storage is the object storage bucket for avatars.
type Storage interface {
	PresignUpload(ctx context.Context, key string, ttl time.Duration, o objectstore.UploadOptions) (objectstore.SignedURL, error)
	PresignDownload(ctx context.Context, key string, ttl time.Duration, o objectstore.DownloadOptions) (objectstore.SignedURL, error)
	Stat(ctx context.Context, key string) (objectstore.ObjectInfo, error)
	ReadHead(ctx context.Context, key string, n int64) ([]byte, error)
	Remove(ctx context.Context, key string) error
}

// AvatarLimits bounds avatar uploads and presigned URL lifetimes.
type AvatarLimits struct {
	MaxSize        int64
	UploadURLTTL   time.Duration
	DownloadURLTTL time.Duration
}

const (
	defaultAvatarMaxSize        int64 = 5 << 20
	defaultAvatarUploadURLTTL         = 15 * time.Minute
	defaultAvatarDownloadURLTTL       = 15 * time.Minute
)

func normalizeAvatarLimits(l AvatarLimits) AvatarLimits {
	if l.MaxSize <= 0 {
		l.MaxSize = defaultAvatarMaxSize
	}
	if l.UploadURLTTL <= 0 {
		l.UploadURLTTL = defaultAvatarUploadURLTTL
	}
	if l.DownloadURLTTL <= 0 {
		l.DownloadURLTTL = defaultAvatarDownloadURLTTL
	}
	return l
}

// CreatedAvatarUpload is a new avatar upload session with its presigned PUT URL.
type CreatedAvatarUpload struct {
	Upload domain.AvatarUpload
	URL    objectstore.SignedURL
}

// Transactor runs fn as one unit of work: a profile change and its event
// (transactional outbox) commit or roll back together.
type Transactor interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type noTx struct{}

func (noTx) InTx(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

// Service exposes the use cases.
type Service struct {
	repo         Repository
	pub          Publisher
	tx           Transactor
	store        Storage
	avatarLimits AvatarLimits
	log          *slog.Logger
	now          func() time.Time
}

// New returns a Service; a nil tx runs without a transaction (tests).
func New(repo Repository, pub Publisher, tx Transactor, log *slog.Logger) *Service {
	if tx == nil {
		tx = noTx{}
	}
	return &Service{
		repo:         repo,
		pub:          pub,
		tx:           tx,
		avatarLimits: normalizeAvatarLimits(AvatarLimits{}),
		log:          log,
		now:          time.Now,
	}
}

// WithAvatarStorage configures optional object storage for avatar uploads and URLs.
func (s *Service) WithAvatarStorage(store Storage, limits AvatarLimits) *Service {
	s.store = store
	s.avatarLimits = normalizeAvatarLimits(limits)
	return s
}

// maxUsernameAttempts bounds suffixing when the derived username is taken.
const maxUsernameAttempts = 20

// CreateForNewUser handles user.registered. At-least-once delivery means it
// may run several times for one user: a second run is a no-op.
func (s *Service) CreateForNewUser(ctx context.Context, userID uuid.UUID, email string) error {
	base := domain.FromRegistration(userID, email, s.now())
	p := base
	for attempt := 1; attempt <= maxUsernameAttempts; attempt++ {
		// One transaction per attempt: a username clash aborts it.
		err := s.tx.InTx(ctx, func(ctx context.Context) error {
			created, err := s.repo.CreateIfAbsent(ctx, p)
			if err != nil || !created {
				return err
			}
			return s.record(ctx, p)
		})
		if errors.Is(err, domain.ErrUsernameTaken) {
			p.Username = base.Username + strconv.Itoa(attempt+1)
			continue
		}
		return err
	}
	return fmt.Errorf("no free username for %s", base.Username)
}

// Get returns a profile.
func (s *Service) Get(ctx context.Context, userID uuid.UUID) (domain.Profile, error) {
	return s.repo.Get(ctx, userID)
}

// Update applies the owner's changes inside a transaction with a row lock so
// concurrent avatar updates are never overwritten or missed in profile.updated.
func (s *Service) Update(ctx context.Context, userID uuid.UUID, c domain.Changes) (domain.Profile, error) {
	var p domain.Profile
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		p, err = s.repo.GetForUpdate(ctx, userID)
		if err != nil {
			return err
		}
		changed, err := p.Apply(c, s.now())
		if err != nil || !changed {
			return err
		}
		if err := s.repo.Update(ctx, p); err != nil {
			return err
		}
		return s.record(ctx, p)
	})
	if err != nil {
		return domain.Profile{}, err
	}
	return p, nil
}

// CreateAvatarUpload opens an avatar upload session and returns a presigned PUT URL.
func (s *Service) CreateAvatarUpload(ctx context.Context, userID uuid.UUID, req domain.AvatarUploadRequest) (CreatedAvatarUpload, error) {
	if s.store == nil {
		return CreatedAvatarUpload{}, domain.ErrStorageUnavailable
	}
	format, sum, err := req.Validate(s.avatarLimits.MaxSize)
	if err != nil {
		return CreatedAvatarUpload{}, err
	}
	if _, err := s.repo.Get(ctx, userID); err != nil {
		return CreatedAvatarUpload{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return CreatedAvatarUpload{}, err
	}
	now := s.now().UTC().Truncate(time.Microsecond)
	u := domain.AvatarUpload{
		ID:          id,
		UserID:      userID,
		ContentType: format.ContentType,
		SizeBytes:   req.SizeBytes,
		SHA256:      sum,
		Key:         media.UserAvatarKey(userID.String(), id.String(), format.Ext),
		Status:      domain.AvatarStatusPending,
		CreatedAt:   now,
		ExpiresAt:   now.Add(s.avatarLimits.UploadURLTTL),
	}
	url, err := s.store.PresignUpload(ctx, u.Key, s.avatarLimits.UploadURLTTL, objectstore.UploadOptions{
		ContentType: u.ContentType,
		SHA256:      sum,
	})
	if err != nil {
		return CreatedAvatarUpload{}, err
	}
	if err := s.repo.CreateAvatarUpload(ctx, u); err != nil {
		return CreatedAvatarUpload{}, err
	}
	return CreatedAvatarUpload{Upload: u, URL: url}, nil
}

// CompleteAvatarUpload verifies the uploaded avatar object, settles the upload
// session, updates the profile's avatar key, and publishes events.
func (s *Service) CompleteAvatarUpload(ctx context.Context, userID, uploadID uuid.UUID) (domain.Profile, error) {
	if s.store == nil {
		return domain.Profile{}, domain.ErrStorageUnavailable
	}
	u, err := s.repo.GetAvatarUpload(ctx, uploadID)
	if err != nil {
		return domain.Profile{}, err
	}
	if u.UserID != userID {
		return domain.Profile{}, domain.ErrAvatarUploadNotFound
	}
	if u.Status != domain.AvatarStatusPending {
		return s.settledAvatarOutcome(ctx, u)
	}
	reason, err := s.verifyAvatar(ctx, u)
	if err != nil {
		return domain.Profile{}, err
	}
	return s.settleAvatar(ctx, u, reason)
}

func (s *Service) settleAvatar(ctx context.Context, u domain.AvatarUpload, reason string) (domain.Profile, error) {
	now := s.now().UTC().Truncate(time.Microsecond)
	settled := u.Settle(reason, now)
	var (
		changed bool
		oldKey  string
		profile domain.Profile
	)
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		if changed, err = s.repo.SettleAvatarUpload(ctx, settled); err != nil || !changed {
			return err
		}
		if reason != "" {
			return nil
		}
		profile, err = s.repo.GetForUpdate(ctx, u.UserID)
		if err != nil {
			return err
		}
		oldKey = profile.AvatarKey
		profile.AvatarKey = settled.Key
		profile.UpdatedAt = now
		if err := s.repo.SetAvatarKey(ctx, u.UserID, settled.Key, now); err != nil {
			return err
		}
		if err := s.record(ctx, profile); err != nil {
			return err
		}
		return s.recordAvatarChanged(ctx, u.UserID, profile.AvatarKey, now)
	})
	if err != nil {
		return domain.Profile{}, err
	}
	if !changed {
		latest, err := s.repo.GetAvatarUpload(ctx, u.ID)
		if err != nil {
			return domain.Profile{}, err
		}
		return s.settledAvatarOutcome(ctx, latest)
	}
	if reason != "" {
		if reason != domain.ReasonExpired {
			s.removeBestEffort(ctx, u.Key, "rejected avatar not removed")
		}
		return domain.Profile{}, &domain.AvatarRejectedError{Reason: reason}
	}
	if oldKey != "" && oldKey != profile.AvatarKey {
		s.removeBestEffort(ctx, oldKey, "previous avatar not removed")
	}
	return profile, nil
}

func (s *Service) settledAvatarOutcome(ctx context.Context, u domain.AvatarUpload) (domain.Profile, error) {
	if u.Status == domain.AvatarStatusFailed {
		return domain.Profile{}, &domain.AvatarRejectedError{Reason: u.FailureReason}
	}
	return s.repo.Get(ctx, u.UserID)
}

func (s *Service) verifyAvatar(ctx context.Context, u domain.AvatarUpload) (string, error) {
	if s.now().After(u.ExpiresAt) {
		return domain.ReasonExpired, nil
	}
	info, err := s.store.Stat(ctx, u.Key)
	if errors.Is(err, objectstore.ErrNotFound) {
		return "", domain.ErrAvatarNotUploaded
	}
	if err != nil {
		return "", err
	}
	switch {
	case info.Size != u.SizeBytes || info.Size > s.avatarLimits.MaxSize:
		return domain.ReasonSizeMismatch, nil
	case info.ContentType != u.ContentType:
		return domain.ReasonContentTypeMismatch, nil
	case info.SHA256 != base64.StdEncoding.EncodeToString(u.SHA256):
		return domain.ReasonChecksumMismatch, nil
	}
	head, err := s.store.ReadHead(ctx, u.Key, domain.AvatarSniffLen)
	if err != nil {
		return "", fmt.Errorf("sniff avatar: %w", err)
	}
	if f, _ := domain.AvatarFormatOf(u.ContentType); !f.Matches(head) {
		return domain.ReasonUnrecognizedFormat, nil
	}
	return "", nil
}

// DeleteAvatar clears the user's avatar key, publishes events, and removes the
// old object best-effort.
func (s *Service) DeleteAvatar(ctx context.Context, userID uuid.UUID) (domain.Profile, error) {
	var (
		p      domain.Profile
		oldKey string
	)
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		p, err = s.repo.GetForUpdate(ctx, userID)
		if err != nil {
			return err
		}
		if p.AvatarKey == "" {
			return nil
		}
		oldKey = p.AvatarKey
		now := s.now().UTC().Truncate(time.Microsecond)
		p.AvatarKey = ""
		p.UpdatedAt = now
		if err := s.repo.SetAvatarKey(ctx, userID, "", now); err != nil {
			return err
		}
		if err := s.record(ctx, p); err != nil {
			return err
		}
		return s.recordAvatarChanged(ctx, userID, "", now)
	})
	if err != nil {
		return domain.Profile{}, err
	}
	if oldKey != "" && s.store != nil {
		s.removeBestEffort(ctx, oldKey, "deleted avatar not removed")
	}
	return p, nil
}

// AvatarURL returns a presigned download URL for avatarKey, or nil when empty,
// storage is unconfigured, or presigning fails.
func (s *Service) AvatarURL(ctx context.Context, avatarKey string) (*string, error) {
	if avatarKey == "" || s.store == nil {
		return nil, nil
	}
	ttl := s.avatarLimits.DownloadURLTTL
	u, err := s.store.PresignDownload(ctx, avatarKey, ttl, objectstore.DownloadOptions{
		CacheControl: "private, max-age=" + strconv.Itoa(int(ttl.Seconds())),
	})
	if err != nil {
		s.log.WarnContext(ctx, "presign avatar download failed", "key", avatarKey, "error", err)
		return nil, nil
	}
	return &u.URL, nil
}

func (s *Service) removeBestEffort(ctx context.Context, key, msg string) {
	if err := s.store.Remove(ctx, key); err != nil {
		s.log.WarnContext(ctx, msg, "key", key, "error", err)
	}
}

// record writes profile.updated in the current unit of work (transactional
// outbox): it is published if and only if the change commits.
func (s *Service) record(ctx context.Context, p domain.Profile) error {
	if err := s.pub.ProfileUpdated(ctx, p); err != nil {
		return fmt.Errorf("record profile.updated: %w", err)
	}
	return nil
}

func (s *Service) recordAvatarChanged(ctx context.Context, userID uuid.UUID, avatarKey string, at time.Time) error {
	if err := s.pub.AvatarChanged(ctx, userID, avatarKey, at); err != nil {
		return fmt.Errorf("record avatar.changed: %w", err)
	}
	return nil
}
