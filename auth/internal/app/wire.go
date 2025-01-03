//go:build wireinject

package app

import (
	"context"
	"fmt"
	"log/slog"
	"mzhn/auth/internal/transport/grpc"
	"mzhn/auth/internal/transport/http"
	"time"

	"mzhn/auth/internal/config"
	"mzhn/auth/internal/services/authservice"
	"mzhn/auth/internal/storage/pg"

	rd "mzhn/auth/internal/storage/redis"

	"github.com/google/wire"
	_ "github.com/jackc/pgx/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

func New() (*App, func(), error) {
	panic(wire.Build(
		newApp,
		_servers,

		pg.NewUserStorage,
		pg.NewRoleStorage,
		rd.NewSessionsStorage,

		wire.Bind(new(authservice.RoleStorage), new(*pg.RoleStorage)),
		wire.Bind(new(authservice.UserSaver), new(*pg.UsersStorage)),
		wire.Bind(new(authservice.UserProvider), new(*pg.UsersStorage)),
		wire.Bind(new(authservice.SessionsStorage), new(*rd.SessionsStorage)),
		authservice.New,

		_pg,
		_redis,
		config.New,
	))
}

func _pg(cfg *config.Config) (*sqlx.DB, func(), error) {
	host := cfg.Pg.Host
	port := cfg.Pg.Port
	user := cfg.Pg.User
	pass := cfg.Pg.Pass
	name := cfg.Pg.Name

	cs := fmt.Sprintf(`postgres://%s:%s@%s:%d/%s?sslmode=disable`, user, pass, host, port, name)

	db, err := sqlx.Connect("pgx", cs)
	if err != nil {
		return nil, nil, err
	}

	slog.Debug("connecting to database", slog.String("conn", cs))
	t := time.Now()
	if err := db.Ping(); err != nil {
		slog.Error("failed to connect to database", slog.String("err", err.Error()), slog.String("conn", cs))
		return nil, func() { db.Close() }, err
	}
	slog.Info("connected to database", slog.String("ping", fmt.Sprintf("%2.fs", time.Since(t).Seconds())))

	return db, func() { db.Close() }, nil
}

func _redis(cfg *config.Config) (*redis.Client, func(), error) {
	host := cfg.Redis.Host
	port := cfg.Redis.Port
	pass := cfg.Redis.Pass

	cs := fmt.Sprintf(`redis://%s:%s@%s:%d`, host, pass, host, port)

	log := slog.With()

	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", host, port),
		Password: pass,
		DB:       0,
	})

	log.Debug("connecting to redis", slog.String("conn", cs))
	t := time.Now()
	if err := client.Ping(context.Background()).Err(); err != nil {
		slog.Error("failed to connect to redis", slog.String("err", err.Error()), slog.String("conn", cs))
		return nil, func() { client.Close() }, err
	}
	log.Info("connected to redis", slog.String("ping", fmt.Sprintf("%2.fs", time.Since(t).Seconds())))

	return client, func() { client.Close() }, nil
}

func _servers(cfg *config.Config, as *authservice.AuthService) []Server {
	servers := make([]Server, 0, 2)

	if cfg.Http.Enabled {
		servers = append(servers, http.New(cfg, as))
	}

	if cfg.Grpc.Enabled {
		servers = append(servers, grpc.New(cfg, as))
	}

	return servers
}
