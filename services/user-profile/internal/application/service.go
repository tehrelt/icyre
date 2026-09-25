// Package application implements User Profile use cases.
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/services/user-profile/internal/domain"
)

// Repository persists profiles.
//
// CreateIfAbsent inserts unless a profile for the user exists (then created
// is false) and returns domain.ErrUsernameTaken on a username clash.
// Update returns domain.ErrUsernameTaken the same way.
type Repository interface {
	CreateIfAbsent(ctx context.Context, p domain.Profile) (created bool, err error)
	Get(ctx context.Context, userID uuid.UUID) (domain.Profile, error)
	Update(ctx context.Context, p domain.Profile) error
}

// Publisher announces profile changes.
type Publisher interface {
	ProfileUpdated(ctx context.Context, p domain.Profile) error
}

// Service exposes the use cases.
type Service struct {
	repo Repository
	pub  Publisher
	log  *slog.Logger
	now  func() time.Time
}

// New returns a Service.
func New(repo Repository, pub Publisher, log *slog.Logger) *Service {
	return &Service{repo: repo, pub: pub, log: log, now: time.Now}
}

// maxUsernameAttempts bounds suffixing when the derived username is taken.
const maxUsernameAttempts = 20

// CreateForNewUser handles user.registered. At-least-once delivery means it
// may run several times for one user: a second run is a no-op.
func (s *Service) CreateForNewUser(ctx context.Context, userID uuid.UUID, email string) error {
	base := domain.FromRegistration(userID, email, s.now())
	p := base
	for attempt := 1; attempt <= maxUsernameAttempts; attempt++ {
		created, err := s.repo.CreateIfAbsent(ctx, p)
		switch {
		case errors.Is(err, domain.ErrUsernameTaken):
			p.Username = base.Username + strconv.Itoa(attempt+1)
			continue
		case err != nil:
			return err
		case created:
			s.publish(ctx, p)
		}
		return nil
	}
	return fmt.Errorf("no free username for %s", base.Username)
}

// Get returns a profile.
func (s *Service) Get(ctx context.Context, userID uuid.UUID) (domain.Profile, error) {
	return s.repo.Get(ctx, userID)
}

// Update applies the owner's changes.
func (s *Service) Update(ctx context.Context, userID uuid.UUID, c domain.Changes) (domain.Profile, error) {
	p, err := s.repo.Get(ctx, userID)
	if err != nil {
		return domain.Profile{}, err
	}
	changed, err := p.Apply(c, s.now())
	if err != nil || !changed {
		return p, err
	}
	if err := s.repo.Update(ctx, p); err != nil {
		return domain.Profile{}, err
	}
	s.publish(ctx, p)
	return p, nil
}

func (s *Service) publish(ctx context.Context, p domain.Profile) {
	if err := s.pub.ProfileUpdated(ctx, p); err != nil {
		s.log.ErrorContext(ctx, "publish profile.updated failed", "error", err)
	}
}
