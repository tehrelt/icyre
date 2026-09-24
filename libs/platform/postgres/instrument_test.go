package postgres

import "testing"

func TestOperation(t *testing.T) {
	cases := map[string]string{
		"SELECT * FROM t":                      "select",
		"  insert into t values ($1)":          "insert",
		"-- name: X\nUPDATE t SET a = 1":       "update",
		"WITH x AS (SELECT 1) SELECT * FROM x": "with",
		"VACUUM":                               "other",
		"-- only a comment":                    "other",
	}
	for sql, want := range cases {
		if got := operation(sql); got != want {
			t.Errorf("operation(%q) = %q, want %q", sql, got, want)
		}
	}
}
