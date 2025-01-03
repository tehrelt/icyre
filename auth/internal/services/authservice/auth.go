package authservice

import (
	"context"
	"errors"
	"log/slog"

	"mzhn/auth/internal/config"
	"mzhn/auth/internal/domain/dto"
	"mzhn/auth/internal/domain/entity"
	"mzhn/auth/internal/storage"
	"mzhn/auth/pkg/sl"
)

//go:generate go run github.com/vektra/mockery/v2@v2.46.0 --name=UserProvider
type UserProvider interface {
	Find(ctx context.Context, slug string) (*entity.User, error)
	Count(ctx context.Context) (int64, error)
}

//go:generate go run github.com/vektra/mockery/v2@v2.46.0 --name=UserSaver
type UserSaver interface {
	Save(ctx context.Context, user *dto.CreateUser) (*entity.User, error)
}

//go:generate go run github.com/vektra/mockery/v2@v2.46.0 --name=SessionsStorage
type SessionsStorage interface {
	Check(ctx context.Context, userId, token string) error
	Save(ctx context.Context, userId, token string) error
	Delete(ctx context.Context, userId string) error
}

//go:generate go run github.com/vektra/mockery/v2@v2.46.0 --name=RoleStorage
type RoleStorage interface {
	Add(ctx context.Context, dto *dto.AddRoles) error
	Remove(ctx context.Context, dto *dto.RemoveRoles) error
}

type AuthService struct {
	userSaver    UserSaver
	userProvider UserProvider
	roles        RoleStorage
	sessions     SessionsStorage
	cfg          *config.Config
	logger       *slog.Logger
}

func New(usaver UserSaver, uprovider UserProvider, r RoleStorage, s SessionsStorage, cfg *config.Config) *AuthService {
	svc := &AuthService{
		cfg:          cfg,
		userSaver:    usaver,
		userProvider: uprovider,
		roles:        r,
		sessions:     s,
		logger:       slog.With(sl.Module("authservice.AuthService")),
	}

	if err := svc.setup(context.Background()); err != nil {
		panic(err)
	}

	return svc
}

func (s *AuthService) setup(ctx context.Context) error {
	fn := "authservice.setup"
	log := s.logger.With(sl.Method(fn))

	log.Info("setup")

	email := s.cfg.DefaultAdmin.Email
	password := s.cfg.DefaultAdmin.Password

	_, err := s.userProvider.Find(ctx, email)
	if err == nil {
		return nil
	}

	if !errors.Is(err, storage.ErrUserNotFound) {
		return err
	}

	log.Info("default user not found")

	if _, err := s.Register(ctx, &dto.CreateUser{
		Email:    email,
		Password: password,
	}); err != nil {
		return err
	}

	user, err := s.userProvider.Find(ctx, email)
	if err != nil {
		return err
	}

	if err := s.roles.Add(ctx, &dto.AddRoles{
		UserId: user.Id,
		Roles:  []entity.Role{entity.RoleAdmin},
	}); err != nil {
		return err
	}

	log.Info("default admin created")

	return nil
}
