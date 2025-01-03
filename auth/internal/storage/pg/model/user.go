package model

import (
	"mzhn/auth/internal/domain/entity"
	"time"
)

type User struct {
	Id             string     `db:"id"`
	LastName       *string    `db:"last_name"`
	FirstName      *string    `db:"first_name"`
	MiddleName     *string    `db:"middle_name"`
	Email          string     `db:"email"`
	HashedPassword string     `db:"hashed_password"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      *time.Time `db:"updated_at"`
}

func (u *User) ToEntity(roles ...entity.Role) *entity.User {
	return &entity.User{
		Id:             u.Id,
		LastName:       u.LastName,
		FirstName:      u.FirstName,
		MiddleName:     u.MiddleName,
		Email:          u.Email,
		HashedPassword: u.HashedPassword,
		Roles:          roles,
		CreatedAt:      u.CreatedAt,
		UpdatedAt:      u.UpdatedAt,
	}
}
