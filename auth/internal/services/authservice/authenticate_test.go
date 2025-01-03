package authservice_test

import (
	"context"
	"errors"
	"mzhn/auth/internal/config"
	"mzhn/auth/internal/domain"
	"mzhn/auth/internal/domain/dto"
	"mzhn/auth/internal/domain/entity"
	"mzhn/auth/internal/lib/jwt"
	"mzhn/auth/internal/services/authservice"
	"mzhn/auth/internal/services/authservice/mocks"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	secret = "secret"
	ttl    = time.Second * 10
)

func authService(t *testing.T, ttl time.Duration) *authservice.AuthService {
	t.Helper()

	usaver := mocks.NewUserSaver(t)
	uprovider := mocks.NewUserProvider(t)
	rolestorage := mocks.NewRoleStorage(t)
	sessionstorage := mocks.NewSessionsStorage(t)

	return authservice.New(usaver, uprovider, rolestorage, sessionstorage, &config.Config{Jwt: config.Jwt{AccessSecret: secret, AccessTTL: int(ttl)}})
}

func generateToken(t *testing.T, id, email string, ttl time.Duration) string {
	t.Helper()

	token, err := jwt.Sign(&entity.UserClaims{Id: id, Email: email}, ttl, []byte(secret))
	require.NoError(t, err)
	require.NotEmpty(t, token)

	return token
}

func TestTokenExpired(t *testing.T) {

	id := uuid.New().String()
	email := "expired@test.com"

	svc := authService(t, ttl)

	token := generateToken(t, id, email, 0)

	_, err := svc.Authenticate(context.TODO(), &dto.Authenticate{AccessToken: token, Roles: []entity.Role{}})
	require.Error(t, err)
	require.True(t, errors.Is(err, domain.ErrTokenExpired))
}

func TestAuthenticateGood(t *testing.T) {

	cases := []struct {
		name       string
		id         string
		email      string
		userRoles  []entity.Role
		checkRoles []entity.Role
		wantError  error
	}{
		{
			name:       "good path",
			id:         "1",
			email:      "test@test.com",
			userRoles:  []entity.Role{entity.RoleAdmin},
			checkRoles: []entity.Role{entity.RoleAdmin},
			wantError:  nil,
		},
		{
			name:       "insufficent permission",
			id:         "1",
			email:      "test@test.com",
			userRoles:  []entity.Role{},
			checkRoles: []entity.Role{entity.RoleAdmin},
			wantError:  domain.ErrInsufficientPermission,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {

			usaver := mocks.NewUserSaver(t)

			uprovider := mocks.NewUserProvider(t)
			uprovider.
				On("Find", context.TODO(), c.id).
				Return(&entity.User{
					Id:    c.id,
					Email: c.email,
					Roles: c.userRoles,
				}, nil)

			rolestorage := mocks.NewRoleStorage(t)
			sessionstorage := mocks.NewSessionsStorage(t)

			svc := authservice.New(usaver, uprovider, rolestorage, sessionstorage, &config.Config{Jwt: config.Jwt{AccessSecret: secret, AccessTTL: int(ttl)}})
			token := generateToken(t, c.id, c.email, ttl)

			u, err := svc.Authenticate(context.TODO(), &dto.Authenticate{AccessToken: token, Roles: c.checkRoles})
			if c.wantError != nil {
				if assert.Error(t, err) {
					require.Equal(t, true, errors.Is(err, c.wantError))
				}
				require.Empty(t, u)
			} else {
				require.NoError(t, err)
				require.NotEmpty(t, u)

				assert.Equal(t, c.id, u.Id)
				assert.Equal(t, c.email, u.Email)
			}
		})
	}
}
