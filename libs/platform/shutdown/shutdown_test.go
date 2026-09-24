package shutdown

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"testing"
)

func TestStackClosesInReverseOrder(t *testing.T) {
	var order []string
	s := NewStack(slog.New(slog.NewTextHandler(io.Discard, nil)))
	s.AddFunc("db", func() { order = append(order, "db") })
	s.Add("kafka", func(context.Context) error { order = append(order, "kafka"); return errors.New("x") })
	s.AddFunc("http", func() { order = append(order, "http") })

	err := s.Close(context.Background())
	if err == nil {
		t.Fatal("expected joined error")
	}
	if !slices.Equal(order, []string{"http", "kafka", "db"}) {
		t.Fatalf("order = %v", order)
	}
}
