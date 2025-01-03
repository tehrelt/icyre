package storage

import "errors"

var (
	ErrUserNotFound        = errors.New("user not found")
	ErrUserAlreadyExists   = errors.New("user already exists")
	ErrSessionNotFound     = errors.New("session not found")
	ErrSessionInvalid      = errors.New("invalid session")
	ErrRoleAlreadyAssigned = errors.New("role already assigned")
)
