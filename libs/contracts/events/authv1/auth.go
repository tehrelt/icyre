// Package authv1 holds version 1 payloads of Auth Service events, published
// to events.TopicAuthEvents with the user ID as key.
package authv1

import "time"

// Version is the eventVersion of every payload in this package.
const Version = 1

// Event types.
const (
	TypeUserRegistered = "user.registered"
	TypeSessionCreated = "session.created"
	TypeSessionRevoked = "session.revoked"
)

// UserRegistered announces a new account. It never carries credentials.
type UserRegistered struct {
	UserID       string    `json:"userId"`
	Email        string    `json:"email"`
	RegisteredAt time.Time `json:"registeredAt"`
}

// SessionCreated is raised on login/registration.
type SessionCreated struct {
	UserID    string    `json:"userId"`
	SessionID string    `json:"sessionId"`
	CreatedAt time.Time `json:"createdAt"`
}

// SessionRevoked is raised on logout, explicit revoke or refresh-token reuse.
type SessionRevoked struct {
	UserID    string    `json:"userId"`
	SessionID string    `json:"sessionId"`
	Reason    string    `json:"reason"` // logout | revoked | token_reuse
	RevokedAt time.Time `json:"revokedAt"`
}
