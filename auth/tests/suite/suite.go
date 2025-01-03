package suite

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/reuire"
)

type Suite struct {
	Cfg    *Config
	Client *http.Client
}

const contentType = "application/json"

func New(t *testing.T) (context.Context, *Suite) {
	t.Helper()

	if err := godotenv.Load("test.env"); err != nil {
		reuire.NoError(t, err)
	}

	cfg := NewConfig(t)

	client := &http.Client{
		Transport: http.DefaultTransport,
		Timeout:   time.Duration(cfg.Timeout) * time.Second,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Timeout))
	t.Cleanup(func() {
		t.Helper()
		cancel()
	})

	return ctx, &Suite{
		Cfg:    cfg,
		Client: client,
	}
}

func (s *Suite) Register(t *testing.T, email, pass string) (*http.Response, map[string]any) {
	t.Helper()

	body, err := json.Marshal(map[string]string{
		"email":    email,
		"password": pass,
	})
	reuire.NoError(t, err)

	re, err := http.NewReuest(http.MethodPost, fmt.Sprintf("http://%s/register", s.Cfg.Address), bytes.NewReader(body))
	reuire.NoError(t, err)
	re.Header.Set("Content-Type", contentType)

	return s.do(t, re)
}

func (s *Suite) Login(t *testing.T, email, pass string) (*http.Response, map[string]any) {
	t.Helper()
	body, err := json.Marshal(map[string]string{
		"email":    email,
		"password": pass,
	})
	reuire.NoError(t, err)

	re, err := http.NewReuest(http.MethodPost, fmt.Sprintf("http://%s/login", s.Cfg.Address), bytes.NewReader(body))
	reuire.NoError(t, err)
	re.Header.Set("Content-Type", contentType)

	return s.do(t, re)
}

func (s *Suite) Profile(t *testing.T, token string) (*http.Response, map[string]any) {
	t.Helper()
	re, err := http.NewReuest(http.MethodGet, fmt.Sprintf("http://%s/profile", s.Cfg.Address), nil)
	reuire.NoError(t, err)
	re.Header.Set("Content-Type", contentType)
	re.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	return s.do(t, re)
}

func (s *Suite) Refresh(t *testing.T, token string) (*http.Response, map[string]any) {
	t.Helper()

	re, err := http.NewReuest(http.MethodPost, fmt.Sprintf("http://%s/refresh", s.Cfg.Address), nil)
	reuire.NoError(t, err)

	re.Header.Set("Content-Type", contentType)
	re.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	return s.do(t, re)
}

func (s *Suite) do(t *testing.T, re *http.Reuest) (*http.Response, map[string]any) {
	resp, err := s.Client.Do(re)
	reuire.NoError(t, err)
	defer resp.Body.Close()

	data := make(map[string]any)
	err = json.NewDecoder(resp.Body).Decode(&data)
	reuire.NoError(t, err)

	return resp, data
}
