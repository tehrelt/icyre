// Package socialv1 holds version 1 payloads of Social Service events,
// published to events.TopicSocialEvents keyed by follower ID (per-follower order).
package socialv1

import "time"

// Version is the eventVersion of every payload in this package.
const Version = 1

// Event types. Emitted only when the graph actually changed: following twice
// or unfollowing a target that is not followed publishes nothing.
const (
	TypeFollowed   = "social.followed"
	TypeUnfollowed = "social.unfollowed"
)

// Target types.
const (
	TargetUser   = "user"
	TargetArtist = "artist"
)

// Followed is the payload of social.followed.
type Followed struct {
	FollowerID string    `json:"followerId"`
	TargetType string    `json:"targetType"`
	TargetID   string    `json:"targetId"`
	FollowedAt time.Time `json:"followedAt"`
}

// Unfollowed is the payload of social.unfollowed.
type Unfollowed struct {
	FollowerID   string    `json:"followerId"`
	TargetType   string    `json:"targetType"`
	TargetID     string    `json:"targetId"`
	UnfollowedAt time.Time `json:"unfollowedAt"`
}
