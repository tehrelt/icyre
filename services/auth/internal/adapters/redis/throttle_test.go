package redis

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

func TestLoginThrottleKeyPrivacyAndDeterminism(t *testing.T) {
	fixed := time.Unix(1_700_000_015, 0) // bucket = 1700000015 / 60 = 28333333, offset = 35s
	th := NewLoginThrottle(nil, 5, time.Minute)
	th.now = func() time.Time { return fixed }

	email := "alice.secret@example.com"
	k1, reset1 := th.key(email)
	k2, reset2 := th.key(email)

	if k1 != k2 || reset1 != reset2 {
		t.Fatalf("key not deterministic: (%q, %v) vs (%q, %v)", k1, reset1, k2, reset2)
	}
	for _, leak := range []string{email, "alice", "secret", "example.com"} {
		if strings.Contains(k1, leak) {
			t.Fatalf("key %q leaks raw email fragment %q", k1, leak)
		}
	}

	sum := sha256.Sum256([]byte(email))
	wantPrefix := "ratelimit:login:" + hex.EncodeToString(sum[:8]) + ":"
	if !strings.HasPrefix(k1, wantPrefix) {
		t.Fatalf("key = %q, want prefix %q", k1, wantPrefix)
	}
	if reset1 <= 0 || reset1 > time.Minute {
		t.Fatalf("resetIn = %v, want in (0, 1m]", reset1)
	}

	otherKey, _ := th.key("bob@example.com")
	if otherKey == k1 {
		t.Fatalf("distinct emails produced identical key %q", k1)
	}

	th.now = func() time.Time { return fixed.Add(time.Minute) }
	nextBucketKey, _ := th.key(email)
	if nextBucketKey == k1 {
		t.Fatalf("next window bucket produced identical key %q", k1)
	}
}

func TestLoginThrottleAllowFailedReset(t *testing.T) {
	srv := newFakeRedisServer(t)
	cl := goredis.NewClient(&goredis.Options{
		Addr:            srv.addr,
		Protocol:        2,
		DisableIdentity: true,
	})
	t.Cleanup(func() { _ = cl.Close() })

	now := time.Unix(1_700_000_000, 0)
	th := NewLoginThrottle(cl, 3, time.Minute)
	th.now = func() time.Time { return now }

	ctx := context.Background()
	email := "user@example.com"

	allowed, resetIn, err := th.Allow(ctx, email)
	if err != nil || !allowed || resetIn <= 0 {
		t.Fatalf("initial Allow = (%v, %v, %v), want (true, >0, nil)", allowed, resetIn, err)
	}

	for i := 0; i < 2; i++ {
		if err := th.Failed(ctx, email); err != nil {
			t.Fatalf("Failed #%d: %v", i+1, err)
		}
		allowed, _, err = th.Allow(ctx, email)
		if err != nil || !allowed {
			t.Fatalf("Allow after %d failures = (%v, %v), want (true, nil)", i+1, allowed, err)
		}
	}

	if err := th.Failed(ctx, email); err != nil {
		t.Fatalf("Failed #3: %v", err)
	}
	allowed, resetIn, err = th.Allow(ctx, email)
	if err != nil || allowed || resetIn <= 0 {
		t.Fatalf("Allow at max failures = (%v, %v, %v), want (false, >0, nil)", allowed, resetIn, err)
	}

	// Verify stored key has TTL set and does not contain raw email.
	key, _ := th.key(email)
	if ttl := srv.ttl(key); ttl != time.Minute {
		t.Fatalf("stored TTL = %v, want %v", ttl, time.Minute)
	}
	for _, k := range srv.keys() {
		if strings.Contains(k, "user") || strings.Contains(k, "example.com") {
			t.Fatalf("redis key %q contains raw email", k)
		}
	}

	// Reset clears the counter.
	if err := th.Reset(ctx, email); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	allowed, _, err = th.Allow(ctx, email)
	if err != nil || !allowed {
		t.Fatalf("Allow after Reset = (%v, %v), want (true, nil)", allowed, err)
	}

	// Exhaust again, then advance into next window bucket.
	for i := 0; i < 3; i++ {
		if err := th.Failed(ctx, email); err != nil {
			t.Fatalf("Failed #%d: %v", i+1, err)
		}
	}
	if allowed, _, err := th.Allow(ctx, email); err != nil || allowed {
		t.Fatalf("expected blocked before window rollover, got (%v, %v)", allowed, err)
	}
	now = now.Add(time.Minute)
	if allowed, _, err := th.Allow(ctx, email); err != nil || !allowed {
		t.Fatalf("expected allowed after window rollover, got (%v, %v)", allowed, err)
	}
}

func TestLoginThrottleRedisErrors(t *testing.T) {
	srv := newFakeRedisServer(t)
	srv.setFail(true)

	cl := goredis.NewClient(&goredis.Options{
		Addr:            srv.addr,
		Protocol:        2,
		DisableIdentity: true,
		MaxRetries:      0,
	})
	t.Cleanup(func() { _ = cl.Close() })

	th := NewLoginThrottle(cl, 3, time.Minute)
	ctx := context.Background()

	// Allow fails open on Redis error so outages do not lock out all users.
	allowed, resetIn, err := th.Allow(ctx, "user@example.com")
	if err == nil || !allowed || resetIn != 0 {
		t.Fatalf("Allow on error = (%v, %v, %v), want (true, 0, non-nil err)", allowed, resetIn, err)
	}
	if err := th.Failed(ctx, "user@example.com"); err == nil {
		t.Fatal("Failed on failing redis expected error")
	}
	if err := th.Reset(ctx, "user@example.com"); err == nil {
		t.Fatal("Reset on failing redis expected error")
	}
}

// fakeRedisServer is a minimal in-process RESP2 server supporting the commands
// used by LoginThrottle (PING, GET, MULTI, INCR, EXPIRE, EXEC, DEL, QUIT).
type fakeRedisServer struct {
	addr string
	mu   sync.Mutex
	fail bool
	data map[string]int64
	ttls map[string]time.Duration
}

func newFakeRedisServer(t *testing.T) *fakeRedisServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &fakeRedisServer{
		addr: ln.Addr().String(),
		data: make(map[string]int64),
		ttls: make(map[string]time.Duration),
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go s.serve(conn)
		}
	}()
	return s
}

func (s *fakeRedisServer) setFail(v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fail = v
}

func (s *fakeRedisServer) ttl(key string) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ttls[key]
}

func (s *fakeRedisServer) keys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.data))
	for k := range s.data {
		out = append(out, k)
	}
	return out
}

func (s *fakeRedisServer) serve(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	r := bufio.NewReader(conn)
	var inTx bool
	var queue [][]string

	for {
		args, err := readRESPArray(r)
		if err != nil {
			return
		}
		if len(args) == 0 {
			continue
		}
		cmd := strings.ToUpper(args[0])
		if inTx && cmd != "EXEC" && cmd != "DISCARD" {
			queue = append(queue, args)
			_, _ = io.WriteString(conn, "+QUEUED\r\n")
			continue
		}
		switch cmd {
		case "MULTI":
			inTx = true
			queue = nil
			_, _ = io.WriteString(conn, "+OK\r\n")
		case "EXEC":
			inTx = false
			var b strings.Builder
			fmt.Fprintf(&b, "*%d\r\n", len(queue))
			for _, q := range queue {
				b.WriteString(s.exec(q))
			}
			queue = nil
			_, _ = io.WriteString(conn, b.String())
		default:
			_, _ = io.WriteString(conn, s.exec(args))
		}
	}
}

func (s *fakeRedisServer) exec(args []string) string {
	cmd := strings.ToUpper(args[0])
	s.mu.Lock()
	fail := s.fail
	s.mu.Unlock()
	if fail && cmd != "PING" && cmd != "QUIT" {
		return "-ERR simulated redis failure\r\n"
	}
	switch cmd {
	case "PING":
		return "+PONG\r\n"
	case "QUIT":
		return "+OK\r\n"
	case "GET":
		s.mu.Lock()
		v, ok := s.data[args[1]]
		s.mu.Unlock()
		if !ok {
			return "$-1\r\n"
		}
		str := strconv.FormatInt(v, 10)
		return fmt.Sprintf("$%d\r\n%s\r\n", len(str), str)
	case "INCR":
		s.mu.Lock()
		s.data[args[1]]++
		v := s.data[args[1]]
		s.mu.Unlock()
		return fmt.Sprintf(":%d\r\n", v)
	case "EXPIRE":
		sec, _ := strconv.ParseInt(args[2], 10, 64)
		s.mu.Lock()
		s.ttls[args[1]] = time.Duration(sec) * time.Second
		s.mu.Unlock()
		return ":1\r\n"
	case "DEL":
		s.mu.Lock()
		var deleted int64
		for _, k := range args[1:] {
			if _, ok := s.data[k]; ok {
				delete(s.data, k)
				delete(s.ttls, k)
				deleted++
			}
		}
		s.mu.Unlock()
		return fmt.Sprintf(":%d\r\n", deleted)
	default:
		return fmt.Sprintf("-ERR unknown command %q\r\n", cmd)
	}
}

func readRESPArray(r *bufio.Reader) ([]string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	if !strings.HasPrefix(line, "*") {
		return strings.Fields(line), nil
	}
	n, err := strconv.Atoi(line[1:])
	if err != nil {
		return nil, err
	}
	args := make([]string, 0, n)
	for i := 0; i < n; i++ {
		hdr, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		hdr = strings.TrimSuffix(strings.TrimSuffix(hdr, "\n"), "\r")
		if !strings.HasPrefix(hdr, "$") {
			return nil, fmt.Errorf("unexpected bulk header %q", hdr)
		}
		sz, err := strconv.Atoi(hdr[1:])
		if err != nil {
			return nil, err
		}
		buf := make([]byte, sz+2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		args = append(args, string(buf[:sz]))
	}
	return args, nil
}
