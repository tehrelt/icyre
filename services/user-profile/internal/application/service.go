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

// Transactor runs fn as one unit of work: a profile change and its event
// (transactional outbox) commit or roll back together.
type Transactor interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type noTx struct{}

func (noTx) InTx(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

// Service exposes the use cases.
type Service struct {
	repo Repository
	pub  Publisher
	tx   Transactor
	log  *slog.Logger
	now  func() time.Time
}

// New returns a Service; a nil tx runs without a transaction (tests).
func New(repo Repository, pub Publisher, tx Transactor, log *slog.Logger) *Service {
	if tx == nil {
		tx = noTx{}
	}
	return &Service{repo: repo, pub: pub, tx: tx, log: log, now: time.Now}
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
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
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

// record writes profile.updated in the current unit of work (transactional
// outbox): it is published if and only if the change commits.
func (s *Service) record(ctx context.Context, p domain.Profile) error {
	if err := s.pub.ProfileUpdated(ctx, p); err != nil {
		return fmt.Errorf("record profile.updated: %w", err)
	}
	return nil
}
