package domain

import "errors"

var (
	ErrEmailTaken             = errors.New("email taken")
	ErrUserNotFound           = errors.New("user not found")
	ErrIncorrectPassword      = errors.New("incorrect password")
	ErrInsufficientPermission = errors.New("insufficient permission")
	ErrTokenExpired           = errors.New("token expired")
	ErrTokenInvalid           = errors.New("token invalid")
)
