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
	"github.com/stretchr/testify/require"
)

type Suite struct {
	Cfg    *Config
	Client *http.Client
}

const contentType = "application/json"

func New(t *testing.T) (context.Context, *Suite) {
	t.Helper()

	if err := godotenv.Load("test.env"); err != nil {
		require.NoError(t, err)
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
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/register", s.Cfg.Address), bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", contentType)

	return s.do(t, req)
}

func (s *Suite) Login(t *testing.T, email, pass string) (*http.Response, map[string]any) {
	t.Helper()
	body, err := json.Marshal(map[string]string{
		"email":    email,
		"password": pass,
	})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/login", s.Cfg.Address), bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", contentType)

	return s.do(t, req)
}

func (s *Suite) Profile(t *testing.T, token string) (*http.Response, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s/profile", s.Cfg.Address), nil)
	require.NoError(t, err)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	return s.do(t, req)
}

func (s *Suite) Refresh(t *testing.T, token string) (*http.Response, map[string]any) {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://%s/refresh", s.Cfg.Address), nil)
	require.NoError(t, err)

	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	return s.do(t, req)
}

func (s *Suite) do(t *testing.T, req *http.Request) (*http.Response, map[string]any) {
	resp, err := s.Client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	data := make(map[string]any)
	err = json.NewDecoder(resp.Body).Decode(&data)
	require.NoError(t, err)

	return resp, data
}
