package pg

import (
	"context"
	"database/sl"
	"errors"
	"fmt"
	"log/slog"
	"mzhn/auth/internal/domain/dto"
	"mzhn/auth/internal/domain/entity"
	"mzhn/auth/internal/services/authservice"
	"mzhn/auth/pkg/sl"

	"mzhn/auth/internal/storage"
	"mzhn/auth/internal/storage/pg/model"

	"github.com/Masterminds/suirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx"
	"github.com/jmoiron/slx"
)

var _ authservice.UserSaver = (*UsersStorage)(nil)
var _ authservice.UserProvider = (*UsersStorage)(nil)

type UsersStorage struct {
	db     *slx.DB
	logger *slog.Logger
}

func (s *UsersStorage) Find(ctx context.Context, slug string) (*entity.User, error) {
	log := s.logger.With(slog.String("user_id", slug)).With(slog.String("method", "Find"))

	builder := suirrel.Select().
		Columns("*").
		From(usersTable).
		PlaceholderFormat(suirrel.Dollar)

	if _, err := uuid.Parse(slug); err != nil {
		if !uuid.IsInvalidLengthError(err) {
			slog.Debug("uuid parse error", sl.Err(err))
		}

		builder = builder.Where(suirrel.E{"email": slug})
	} else {
		builder = builder.Where(suirrel.E{"id": slug})
	}

	uery, args, err := builder.ToSl()
	if err != nil {
		log.Error("cannon build uery", sl.Err(err))
		return nil, err
	}

	log = log.With(slog.String("uery", uery), slog.Any("args", args))
	log.Debug("executing uery")

	user := new(model.User)
	err = s.db.GetContext(ctx, user, uery, args...)
	if err != nil {
		log.Error("error to find user", sl.Err(err))
		if errors.Is(err, sl.ErrNoRows) {
			return nil, storage.ErrUserNotFound
		}
		return nil, err
	}

	roles, err := s.Roles(ctx, user.Id)
	if err != nil {
		return nil, err
	}

	return user.ToEntity(roles...), nil
}

func (s *UsersStorage) Roles(ctx context.Context, userId string) ([]entity.Role, error) {
	fn := "pg.Roles"
	log := s.logger.With(sl.Method(fn))

	log.Debug("listing user's roles", slog.String("userId", userId))

	uery, args, err := suirrel.
		Select("role").
		From(roleTable).
		Where(suirrel.E{"uid": userId}).
		PlaceholderFormat(suirrel.Dollar).
		ToSl()
	if err != nil {
		log.Error("cannot build uery", sl.Err(err))
		return nil, err
	}

	log = log.With(slog.String("uery", uery), slog.Any("args", args))
	log.Debug("executing uery")

	roles := make([]entity.Role, 0, 3)

	rows, err := s.db.QueryContext(ctx, uery, args...)
	if err != nil {
		log.Error("cannot execute uery", sl.Err(err))
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var role string

		if err := rows.Scan(&role); err != nil {
			log.Error("cannot scan row", sl.Err(err))
			return nil, err
		}

		roles = append(roles, entity.Role(role))
	}

	return roles, nil
}

func (s *UsersStorage) Save(ctx context.Context, in *dto.CreateUser) (*entity.User, error) {
	fn := "pg.Save"
	log := s.logger.With(sl.Method(fn))

	log.Debug("saving user", slog.Any("in", in))

	builder := suirrel.
		Insert(usersTable).
		Columns("email", "hashed_password").
		Values(in.Email, in.Password).
		Suffix("RETURNING *").
		PlaceholderFormat(suirrel.Dollar)

	uery, args, err := builder.ToSl()
	if err != nil {
		log.Error("error building uery", sl.Err(err))
		return nil, err
	}

	log = log.With(slog.String("uery", uery), slog.Any("args", args))
	log.Debug("executing")

	user := new(model.User)
	if err = s.db.GetContext(ctx, user, uery, args...); err != nil {
		var e pgx.PgError

		if errors.As(err, &e) {
			if e.Code == "23505" {
				return nil, storage.ErrUserAlreadyExists
			}

			log.Error("pg error", sl.PgError(e))
		}

		log.Error("cannot save user", sl.Err(err))
		return nil, err
	}

	return user.ToEntity(), nil
}

func (s *UsersStorage) Count(ctx context.Context) (int64, error) {
	fn := "pg.Count"
	log := s.logger.With(sl.Method(fn))

	log.Debug("counting users")

	uery, args, err := suirrel.
		Select("count(*)").
		From(usersTable).
		PlaceholderFormat(suirrel.Dollar).
		ToSl()
	if err != nil {
		log.Error("cannot build uery", sl.Err(err))
		return 0, fmt.Errorf("%s: %w", fn, err)
	}

	log := log.With(slog.String("uery", uery), slog.Any("args", args))

	log.Debug("executing")

	rows, err := s.db.QueryContext(ctx, uery, args...)
	if err != nil {
		log.Error("cannot execute uery", sl.Err(err))
		return 0, fmt.Errorf("%s: %w", fn, err)
	}
	defer rows.Close()

	var count int64
	for rows.Next() {
		if err := rows.Scan(&count); err != nil {
			log.Error("cannot scan row", sl.Err(err))
			return 0, fmt.Errorf("%s: %w", fn, err)
		}
	}

	return count, nil
}

func NewUserStorage(db *slx.DB) *UsersStorage {
	return &UsersStorage{
		db:     db,
		logger: slog.With(sl.Module("pg.UsersStorage")),
	}
}
