package application

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/tehrelt/icyre/libs/contracts/events/mediav1"
	"github.com/tehrelt/icyre/libs/contracts/media"
)

type fakeStore struct {
	variants    map[media.Quality]Stored
	sum         string
	downloadErr error
	downloads   int
	puts        int
	putErr      error
}

func (f *fakeStore) Bucket() string { return "icyre-media" }

func (f *fakeStore) Download(_ context.Context, _, _, path string) (string, error) {
	f.downloads++
	if f.downloadErr != nil {
		return "", f.downloadErr
	}
	return f.sum, os.WriteFile(path, []byte("master"), 0o600)
}

func (f *fakeStore) Variant(_ context.Context, _ string, q media.Quality) (Stored, bool, error) {
	v, ok := f.variants[q]
	return v, ok, nil
}

func (f *fakeStore) PutVariant(_ context.Context, trackID string, q media.Quality, path string, p Provenance) (Stored, error) {
	if f.putErr != nil {
		return Stored{}, f.putErr
	}
	if _, err := os.Stat(path); err != nil {
		return Stored{}, err
	}
	f.puts++
	v := Stored{Provenance: p, Quality: q, Key: media.TrackAudioKey(trackID, q), SizeBytes: int64(q) * 10}
	if f.variants == nil {
		f.variants = map[media.Quality]Stored{}
	}
	f.variants[q] = v
	return v, nil
}

type fakeEncoder struct {
	probeErr  error
	encodeErr error
	encodes   int
}

func (f *fakeEncoder) Probe(context.Context, string) (int64, error) { return 180_000, f.probeErr }

func (f *fakeEncoder) Encode(_ context.Context, _ string, dst map[media.Quality]string) error {
	f.encodes++
	if f.encodeErr != nil {
		return f.encodeErr
	}
	for _, p := range dst {
		if err := os.WriteFile(p, []byte("aac"), 0o600); err != nil {
			return err
		}
	}
	return nil
}

type fakePub struct {
	transcoded []mediav1.TrackTranscoded
	failed     []mediav1.TranscodeFailed
	err        error
}

func (f *fakePub) Transcoded(_ context.Context, e mediav1.TrackTranscoded) error {
	f.transcoded = append(f.transcoded, e)
	return f.err
}

func (f *fakePub) Failed(_ context.Context, e mediav1.TranscodeFailed) error {
	f.failed = append(f.failed, e)
	return f.err
}

var uploadedAt = time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)

func job() mediav1.TrackUploaded {
	return mediav1.TrackUploaded{UploadID: "u1", TrackID: "t1", Bucket: "icyre-media", Key: "tracks/t1/original/u1.flac",
		SHA256: "aa", UploadedAt: uploadedAt}
}

func setup(t *testing.T) (*Transcoder, *fakeStore, *fakeEncoder, *fakePub) {
	t.Helper()
	st, enc, pub := &fakeStore{sum: "aa"}, &fakeEncoder{}, &fakePub{}
	x := New(st, enc, pub, t.TempDir(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	return x, st, enc, pub
}

func TestTranscodesAndAnnounces(t *testing.T) {
	x, st, _, pub := setup(t)
	out, err := x.Handle(context.Background(), job())
	if err != nil || out != OutcomeTranscoded {
		t.Fatal(out, err)
	}
	if st.puts != 3 || len(pub.transcoded) != 1 {
		t.Fatalf("puts %d, events %d", st.puts, len(pub.transcoded))
	}
	e := pub.transcoded[0]
	if e.TrackID != "t1" || e.UploadID != "u1" || e.DurationMs != 180_000 || e.Codec != "aac" || e.SourceSHA256 != "aa" || len(e.Variants) != 3 {
		t.Fatalf("%+v", e)
	}
	for i, q := range media.Qualities {
		if v := e.Variants[i]; v.Quality != int(q) || v.Key != media.TrackAudioKey("t1", q) || v.ContentType != media.AudioContentType {
			t.Fatalf("variant %d: %+v", i, v)
		}
	}
	if st.variants[media.Quality128].UploadedAt != uploadedAt {
		t.Fatal("provenance not stored")
	}
}

func TestRedeliveryReannouncesWithoutWork(t *testing.T) {
	x, st, enc, pub := setup(t)
	if _, err := x.Handle(context.Background(), job()); err != nil {
		t.Fatal(err)
	}
	out, err := x.Handle(context.Background(), job())
	if err != nil || out != OutcomeDuplicate {
		t.Fatal(out, err)
	}
	if st.downloads != 1 || enc.encodes != 1 || st.puts != 3 {
		t.Fatalf("downloads %d, encodes %d, puts %d", st.downloads, enc.encodes, st.puts)
	}
	if len(pub.transcoded) != 2 || pub.transcoded[1].DurationMs != 180_000 || len(pub.transcoded[1].Variants) != 3 {
		t.Fatalf("%+v", pub.transcoded)
	}
}

func TestStaleUploadIsDropped(t *testing.T) {
	x, st, enc, pub := setup(t)
	newer := Stored{Provenance: Provenance{UploadID: "u2", SourceSHA256: "bb", UploadedAt: uploadedAt.Add(time.Minute)}}
	st.variants = map[media.Quality]Stored{media.Quality64: newer}
	out, err := x.Handle(context.Background(), job())
	if err != nil || out != OutcomeStale || enc.encodes != 0 || len(pub.transcoded) != 0 {
		t.Fatal(out, err)
	}
}

func TestReuploadAndPartialRunAreRedone(t *testing.T) {
	for name, variants := range map[string]map[media.Quality]Stored{
		"older master": {
			media.Quality64:  {Provenance: Provenance{SourceSHA256: "old", UploadedAt: uploadedAt.Add(-time.Hour)}},
			media.Quality128: {Provenance: Provenance{SourceSHA256: "old", UploadedAt: uploadedAt.Add(-time.Hour)}},
			media.Quality256: {Provenance: Provenance{SourceSHA256: "old", UploadedAt: uploadedAt.Add(-time.Hour)}},
		},
		"partial run":  {media.Quality64: {Provenance: Provenance{SourceSHA256: "aa", UploadedAt: uploadedAt}}},
		"seed variant": {media.Quality128: {}},
	} {
		t.Run(name, func(t *testing.T) {
			x, st, enc, _ := setup(t)
			st.variants = variants
			out, err := x.Handle(context.Background(), job())
			if err != nil || out != OutcomeTranscoded || enc.encodes != 1 || st.puts != 3 {
				t.Fatal(out, err)
			}
		})
	}
}

func TestUnusableMasterIsReportedNotRetried(t *testing.T) {
	cases := map[string]struct {
		setup  func(*fakeStore, *fakeEncoder)
		reason string
	}{
		"missing":   {func(s *fakeStore, _ *fakeEncoder) { s.downloadErr = ErrSourceMissing }, mediav1.ReasonSourceMissing},
		"corrupted": {func(s *fakeStore, _ *fakeEncoder) { s.sum = "zz" }, mediav1.ReasonSourceCorrupted},
		"not audio": {func(_ *fakeStore, e *fakeEncoder) { e.probeErr = ErrUndecodable }, mediav1.ReasonUndecodable},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			x, st, enc, pub := setup(t)
			c.setup(st, enc)
			out, err := x.Handle(context.Background(), job())
			if err != nil || out != OutcomeFailed {
				t.Fatal(out, err)
			}
			if len(pub.failed) != 1 || pub.failed[0].Reason != c.reason || pub.failed[0].TrackID != "t1" || st.puts != 0 {
				t.Fatalf("%+v", pub.failed)
			}
		})
	}
}

func TestTransientErrorsAreReturned(t *testing.T) {
	boom := errors.New("boom")
	cases := map[string]func(*fakeStore, *fakeEncoder, *fakePub){
		"download": func(s *fakeStore, _ *fakeEncoder, _ *fakePub) { s.downloadErr = boom },
		"probe":    func(_ *fakeStore, e *fakeEncoder, _ *fakePub) { e.probeErr = boom },
		"encode":   func(_ *fakeStore, e *fakeEncoder, _ *fakePub) { e.encodeErr = boom },
		"upload":   func(s *fakeStore, _ *fakeEncoder, _ *fakePub) { s.putErr = boom },
		"publish":  func(_ *fakeStore, _ *fakeEncoder, p *fakePub) { p.err = boom },
	}
	for name, set := range cases {
		t.Run(name, func(t *testing.T) {
			x, st, enc, pub := setup(t)
			set(st, enc, pub)
			if _, err := x.Handle(context.Background(), job()); !errors.Is(err, boom) {
				t.Fatal(err)
			}
		})
	}
}

func TestIncompleteJobIsInvalid(t *testing.T) {
	x, _, _, _ := setup(t)
	j := job()
	j.SHA256 = ""
	if _, err := x.Handle(context.Background(), j); !errors.Is(err, ErrInvalidJob) {
		t.Fatal(err)
	}
}

func TestWorkDirIsCleanedUp(t *testing.T) {
	dir := t.TempDir()
	x := New(&fakeStore{sum: "aa"}, &fakeEncoder{}, &fakePub{}, dir, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if _, err := x.Handle(context.Background(), job()); err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("left behind: %v", entries)
	}
}
