package pg

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"mzhn/auth/internal/domain/dto"
	"mzhn/auth/internal/services/authservice"
	"mzhn/auth/pkg/sl"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx"
	"github.com/jmoiron/sqlx"
)

var _ authservice.RoleStorage = (*RoleStorage)(nil)

type RoleStorage struct {
	db     *sqlx.DB
	logger *slog.Logger
}

func NewRoleStorage(db *sqlx.DB) *RoleStorage {
	return &RoleStorage{
		db:     db,
		logger: slog.Default().With(slog.String("struct", "RoleStorage")),
	}
}

func (r *RoleStorage) Add(ctx context.Context, dto *dto.AddRoles) (err error) {

	fn := "pg.RoleStorage.Add"
	log := r.logger.With(sl.Method(fn))
	log.Debug("dto", slog.Any("dto", dto))

	tx, err := r.db.Begin()
	if err != nil {
		log.Error("cannot begin transaction", sl.Err(err))
		return err
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
			return
		}

		if commitErr := tx.Commit(); err != nil {
			log.Error("failed to commit transaction", sl.Err(err))
			err = fmt.Errorf("%s: %w", fn, commitErr)
		}
	}()

	for _, role := range dto.Roles {

		if !role.Valid() {
			log.Warn("invalid role", slog.String("role", role.String()))
			continue
		}

		query, args, err := squirrel.
			Insert(roleTable).
			Columns("uid", "role").
			Values(dto.UserId, role).
			PlaceholderFormat(squirrel.Dollar).
			ToSql()
		if err != nil {
			log.Error("cannot build query", sl.Err(err))
			return err
		}

		log = log.With(slog.String("query", query), slog.Any("args", args))

		log.Debug("executing query")
		if _, err := tx.Exec(query, args...); err != nil {
			var e pgx.PgError
			if errors.As(err, &e) {
				if e.Code == "23505" {
					continue
				}

				log.Error("cannot execute query", sl.PgError(e))
				return err
			}
		}
	}

	return nil
}

// func (r *RoleStorage) check(ctx context.Context, userId string, role entity.Role) error {
// 	fn := "pg.RoleStorage.check"
// 	log := r.logger.With(sl.Method(fn))

// 	query, args, err := squirrel.
// 		Select("*").
// 		From(roleTable).
// 		Where(squirrel.Eq{"uid": userId, "role": role}).
// 		PlaceholderFormat(squirrel.Dollar).
// 		ToSql()
// 	if err != nil {
// 		log.Error("cannot build query", sl.Err(err))
// 		return fmt.Errorf("%s: %w", fn, err)
// 	}

// 	qlog := log.With(slog.String("query", query), slog.Any("args", args))
// 	qlog.Debug("executing query")

// 	_, err = r.db.QueryContext(ctx, query, args...)
// 	if err == nil {
// 		log.Debug("role already assigned")
// 		return fmt.Errorf("%s: %w", fn, storage.ErrRoleAlreadyAssigned)
// 	}

// 	if !errors.Is(err, sql.ErrNoRows) {
// 		log.Error("cannot execute query", sl.Err(err))
// 		return fmt.Errorf("%s: %w", fn, err)
// 	}

// 	return nil
// }

func (r *RoleStorage) Remove(ctx context.Context, dto *dto.RemoveRoles) (err error) {
	log := r.logger.With(slog.String("method", "Remove"))

	log.Debug("dto", slog.Any("dto", dto))

	tx, err := r.db.Begin()
	if err != nil {
		log.Error("cannot begin transaction", sl.Err(err))
		return err
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}

		if err := tx.Commit(); err != nil {
			log.Error("cannot commit transaction", sl.Err(err))
		}
	}()

	for _, role := range dto.Roles {
		query, args, err := squirrel.
			Delete(roleTable).
			Where(squirrel.Eq{"uid": dto.UserId, "role": role}).
			PlaceholderFormat(squirrel.Dollar).
			ToSql()
		if err != nil {
			log.Error("cannot build query", sl.Err(err))
			return err
		}

		qlog := log.With(slog.String("query", query), slog.Any("args", args))

		qlog.Debug("executing query")

		if _, err := tx.Exec(query, args...); err != nil {
			qlog.Error("cannot execute query", sl.Err(err))
			return err
		}

	}

	return nil
}
