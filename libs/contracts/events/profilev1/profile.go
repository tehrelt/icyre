// Package profilev1 holds version 1 payloads of User Profile events,
// published to events.TopicProfileEvents with the user ID as key.
package profilev1

import "time"

// Version is the eventVersion of every payload in this package.
const Version = 1

// TypeProfileUpdated is raised after a profile changes (including creation).
const TypeProfileUpdated = "profile.updated"

// TypeAvatarChanged is raised when a user's avatar is set, replaced, or removed.
const TypeAvatarChanged = "avatar.changed"

// ProfileUpdated carries the public state of the profile after the change.
type ProfileUpdated struct {
	UserID      string    `json:"userId"`
	Username    string    `json:"username"`
	DisplayName string    `json:"displayName"`
	AvatarKey   string    `json:"avatarKey,omitempty"`
	Country     string    `json:"country,omitempty"`
	Language    string    `json:"language,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// AvatarChanged carries the updated avatar key (empty when removed).
type AvatarChanged struct {
	UserID    string    `json:"userId"`
	AvatarKey string    `json:"avatarKey,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
}
