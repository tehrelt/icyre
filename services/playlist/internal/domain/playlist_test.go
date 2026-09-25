package domain

import (
	"strings"
	"testing"
)

func TestNormalizeTitle(t *testing.T) {
	if got, err := NormalizeTitle("  Late   night drives "); err != nil || got != "Late night drives" {
		t.Fatal(got, err)
	}
	for _, bad := range []string{"", "   ", strings.Repeat("я", MaxTitle+1)} {
		if _, err := NormalizeTitle(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
