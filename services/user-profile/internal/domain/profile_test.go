package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestFromRegistration(t *testing.T) {
	cases := map[string][2]string{
		"rin.aoki@example.com": {"rin.aoki", "Rin Aoki"},
		"Nova_Hale+music@x.io": {"nova_halemusic", "Nova Hale Music"},
		"ab@x.io":              {"listener", "Ab"},
		"__weird..name__@x.io": {"weird.name", "Weird Name"},
	}
	for email, want := range cases {
		p := FromRegistration(uuid.New(), email, time.Now())
		if p.Username != want[0] || p.DisplayName != want[1] {
			t.Errorf("%s → %q / %q, want %q / %q", email, p.Username, p.DisplayName, want[0], want[1])
		}
	}
}

func TestApply(t *testing.T) {
	p := FromRegistration(uuid.New(), "rin@example.com", time.Now())
	later := time.Now().Add(time.Hour)
	name, country, lang := "  Rin Aoki ", "jp", "ja"
	changed, err := p.Apply(Changes{DisplayName: &name, Country: &country, Language: &lang}, later)
	if err != nil || !changed || p.DisplayName != "Rin Aoki" || p.Country != "JP" || !p.UpdatedAt.Equal(later.Truncate(time.Microsecond)) {
		t.Fatalf("changed=%v err=%v p=%+v", changed, err, p)
	}
	if changed, _ := p.Apply(Changes{DisplayName: &name}, later.Add(time.Hour)); changed {
		t.Fatal("no-op update reported a change")
	}

	bad, empty, badLang := "-no", "", "english"
	_, err = p.Apply(Changes{Username: &bad, DisplayName: &empty, Language: &badLang}, later)
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Fields["username"] == "" || ve.Fields["displayName"] == "" || ve.Fields["language"] == "" {
		t.Fatalf("validation = %v", err)
	}
	if p.DisplayName != "Rin Aoki" {
		t.Fatal("failed update mutated the profile")
	}
}
