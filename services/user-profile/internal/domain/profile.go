// Package domain holds the public profile of a listener.
package domain

import (
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Errors.
var (
	ErrProfileNotFound = errors.New("profile not found")
	ErrUsernameTaken   = errors.New("username is taken")
)

// ValidationError lists invalid fields.
type ValidationError struct{ Fields map[string]string }

func (e *ValidationError) Error() string { return "validation failed" }

var (
	usernamePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._]{1,28}[a-z0-9])$`)
	countryPattern  = regexp.MustCompile(`^[A-Z]{2}$`)
	languagePattern = regexp.MustCompile(`^[a-z]{2,3}(-[A-Z]{2})?$`)
	usernameStrip   = regexp.MustCompile(`[^a-z0-9._]+`)
	usernameRepeats = regexp.MustCompile(`[._]{2,}`)
)

// Limits.
const (
	MaxDisplayName = 50
	MaxBio         = 300
)

// Profile is the public identity of a user (owned by User Profile Service;
// the user ID comes from Auth).
type Profile struct {
	UserID      uuid.UUID
	Username    string
	DisplayName string
	AvatarKey   string
	Bio         string
	Country     string
	Language    string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// FromRegistration builds the initial profile of a new account: username and
// display name come from the email's local part, the listener edits them later.
func FromRegistration(userID uuid.UUID, email string, now time.Time) Profile {
	local, _, _ := strings.Cut(strings.ToLower(email), "@")
	username := usernameStrip.ReplaceAllString(local, "")
	username = usernameRepeats.ReplaceAllString(username, ".")
	username = strings.Trim(username, "._")
	if len(username) > 24 {
		username = username[:24]
	}
	if len(username) < 3 {
		username = "listener"
	}
	display := strings.TrimSpace(strings.Map(func(r rune) rune {
		if r == '.' || r == '_' || r == '-' || r == '+' {
			return ' '
		}
		return r
	}, local))
	if display == "" {
		display = username
	}
	display = capitalize(truncate(display, MaxDisplayName))
	now = stamp(now)
	return Profile{UserID: userID, Username: username, DisplayName: display, CreatedAt: now, UpdatedAt: now}
}

// Changes is a partial update; nil fields stay unchanged.
type Changes struct {
	Username    *string
	DisplayName *string
	Bio         *string
	Country     *string
	Language    *string
}

// Apply validates and applies changes. It reports whether anything changed.
func (p *Profile) Apply(c Changes, now time.Time) (bool, error) {
	next := *p
	fields := map[string]string{}
	if c.Username != nil {
		u := strings.ToLower(strings.TrimSpace(*c.Username))
		if !usernamePattern.MatchString(u) || strings.Contains(u, "..") {
			fields["username"] = "3–30 characters: a–z, 0–9, dots and underscores, not at the ends"
		}
		next.Username = u
	}
	if c.DisplayName != nil {
		d := strings.TrimSpace(*c.DisplayName)
		if d == "" || utf8.RuneCountInString(d) > MaxDisplayName {
			fields["displayName"] = "must be 1–50 characters"
		}
		next.DisplayName = d
	}
	if c.Bio != nil {
		b := strings.TrimSpace(*c.Bio)
		if utf8.RuneCountInString(b) > MaxBio {
			fields["bio"] = "must be at most 300 characters"
		}
		next.Bio = b
	}
	if c.Country != nil {
		v := strings.ToUpper(strings.TrimSpace(*c.Country))
		if v != "" && !countryPattern.MatchString(v) {
			fields["country"] = "must be an ISO 3166-1 alpha-2 code"
		}
		next.Country = v
	}
	if c.Language != nil {
		v := strings.TrimSpace(*c.Language)
		if v != "" && !languagePattern.MatchString(v) {
			fields["language"] = "must be a language tag like en or pt-BR"
		}
		next.Language = v
	}
	if len(fields) > 0 {
		return false, &ValidationError{Fields: fields}
	}
	changed := next.Username != p.Username || next.DisplayName != p.DisplayName || next.Bio != p.Bio || next.Country != p.Country || next.Language != p.Language
	if changed {
		next.UpdatedAt = stamp(now)
		*p = next
	}
	return changed, nil
}

// stamp matches PostgreSQL timestamptz precision so responses agree with reads.
func stamp(t time.Time) time.Time { return t.UTC().Truncate(time.Microsecond) }

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

func capitalize(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		r := []rune(w)
		r[0] = unicode.ToUpper(r[0])
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}
