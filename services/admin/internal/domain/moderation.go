package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"maps"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Actor represents the authenticated caller performing an admin action.
type Actor struct {
	ID        uuid.UUID
	Moderator bool
	Admin     bool
}

// CanModerateTracks reports whether the actor can moderate tracks and view jobs.
func (a Actor) CanModerateTracks() bool { return a.Moderator || a.Admin }

// CanModerateUsers reports whether the actor can moderate users.
func (a Actor) CanModerateUsers() bool { return a.Admin }

// CanViewAuditLogs reports whether the actor can query audit logs.
func (a Actor) CanViewAuditLogs() bool { return a.Admin }

// CanViewOperation reports whether the actor can inspect op.
func (a Actor) CanViewOperation(op Operation) bool {
	return a.Admin || (a.Moderator && op.ActorID == a.ID)
}

// Action identifies a moderation command.
type Action string

const (
	ActionTrackBlock   Action = "track.block"
	ActionTrackUnblock Action = "track.unblock"
	ActionUserBlock    Action = "user.block"
	ActionUserUnblock  Action = "user.unblock"
)

// Valid reports whether a is a supported moderation action.
func (a Action) Valid() bool {
	switch a {
	case ActionTrackBlock, ActionTrackUnblock, ActionUserBlock, ActionUserUnblock:
		return true
	default:
		return false
	}
}

// TargetType identifies the moderated entity kind.
type TargetType string

const (
	TargetTrack TargetType = "track"
	TargetUser  TargetType = "user"
)

// Valid reports whether t is a supported target type.
func (t TargetType) Valid() bool {
	return t == TargetTrack || t == TargetUser
}

// OperationStatus tracks the lifecycle of a moderation operation.
type OperationStatus string

const (
	StatusPending  OperationStatus = "PENDING"
	StatusApplied  OperationStatus = "APPLIED"
	StatusNoop     OperationStatus = "NOOP"
	StatusRejected OperationStatus = "REJECTED"
)

// Settled reports whether the operation has reached a terminal state.
func (s OperationStatus) Settled() bool {
	return s == StatusApplied || s == StatusNoop || s == StatusRejected
}

// AuditPhase records the stage of a moderation operation in the audit log.
type AuditPhase string

const (
	PhaseRequested AuditPhase = "requested"
	PhaseApplied   AuditPhase = "applied"
	PhaseNoop      AuditPhase = "noop"
	PhaseRejected  AuditPhase = "rejected"
)

// Valid reports whether p is a supported audit phase.
func (p AuditPhase) Valid() bool {
	switch p {
	case PhaseRequested, PhaseApplied, PhaseNoop, PhaseRejected:
		return true
	default:
		return false
	}
}

// Operation represents a durable, idempotent moderation command.
type Operation struct {
	ID                 uuid.UUID
	ActorID            uuid.UUID
	IdempotencyKey     string
	RequestHash        string
	Action             Action
	TargetType         TargetType
	TargetID           uuid.UUID
	DesiredBlocked     bool
	Reason             string
	Status             OperationStatus
	ResultCode         string
	BeforeState        map[string]any
	AfterState         map[string]any
	RevocationsPending bool
	RequestID          string
	CreatedAt          time.Time
	UpdatedAt          time.Time
	CompletedAt        *time.Time
}

// Settle returns a new Operation copy transitioned to a terminal state.
func (o Operation) Settle(status OperationStatus, resultCode string, before, after map[string]any, revocationsPending bool, at time.Time) Operation {
	ts := at.UTC()
	next := o
	next.Status = status
	next.ResultCode = resultCode
	next.BeforeState = CloneMap(before)
	next.AfterState = CloneMap(after)
	next.RevocationsPending = revocationsPending
	next.UpdatedAt = ts
	next.CompletedAt = &ts
	return next
}

// AuditEntry is an immutable audit log record.
type AuditEntry struct {
	ID          uuid.UUID
	OperationID uuid.UUID
	ActorID     uuid.UUID
	Action      Action
	TargetType  TargetType
	TargetID    uuid.UUID
	Phase       AuditPhase
	Reason      string
	Metadata    map[string]any
	RequestID   string
	CreatedAt   time.Time
}

// TrackModerationReceipt is the owner response from Catalog moderation.
type TrackModerationReceipt struct {
	OperationID    uuid.UUID
	TrackID        uuid.UUID
	PreviousStatus string
	CurrentStatus  string
	Changed        bool
	CreatedAt      time.Time
}

// UserModerationReceipt is the owner response from Auth moderation.
type UserModerationReceipt struct {
	OperationID        uuid.UUID
	UserID             uuid.UUID
	Blocked            bool
	Changed            bool
	RevokedSessions    int
	RevocationsPending bool
	CreatedAt          time.Time
}

// ValidateModerationInput validates idempotencyKey (1..128 chars), reason
// (trimmed 1..512 chars), and targetID.
func ValidateModerationInput(idempotencyKey string, targetID uuid.UUID, reason string) (string, string, error) {
	fields := map[string]string{}
	key := strings.TrimSpace(idempotencyKey)
	if n := utf8.RuneCountInString(key); n < 1 || n > 128 {
		fields["Idempotency-Key"] = "must be between 1 and 128 characters"
	}
	if targetID == uuid.Nil {
		fields["targetId"] = "must be a valid UUID"
	}
	trimmedReason := strings.TrimSpace(reason)
	if n := utf8.RuneCountInString(trimmedReason); n < 1 || n > 512 {
		fields["reason"] = "must be between 1 and 512 characters"
	}
	if len(fields) > 0 {
		return "", "", &ValidationError{Fields: fields}
	}
	return key, trimmedReason, nil
}

// ComputeRequestHash returns a deterministic SHA-256 hex digest of the
// canonical moderation request parameters.
func ComputeRequestHash(action Action, targetType TargetType, targetID uuid.UUID, desiredBlocked bool, reason string) string {
	payload, _ := json.Marshal(struct {
		Action         Action     `json:"action"`
		TargetType     TargetType `json:"targetType"`
		TargetID       string     `json:"targetId"`
		DesiredBlocked bool       `json:"desiredBlocked"`
		Reason         string     `json:"reason"`
	}{
		Action:         action,
		TargetType:     targetType,
		TargetID:       targetID.String(),
		DesiredBlocked: desiredBlocked,
		Reason:         reason,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

// CloneMap returns a shallow copy of m, or an empty non-nil map when m is nil.
func CloneMap(m map[string]any) map[string]any {
	if len(m) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(m))
	maps.Copy(out, m)
	return out
}
