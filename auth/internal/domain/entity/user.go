package entity

import (
	"time"
)

type User struct {
	Id             string
	LastName       *string
	FirstName      *string
	MiddleName     *string
	Email          string
	HashedPassword string
	Roles          []Role
	CreatedAt      time.Time
	UpdatedAt      *time.Time
}

type UserClaims struct {
	Id    string
	Email string
}
