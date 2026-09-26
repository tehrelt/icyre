package application

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tehrelt/icyre/libs/contracts/events/mediav1"
	"github.com/tehrelt/icyre/workers/audio-analysis/internal/domain"
)

type fakeSource struct {
	err  error
	gets int
}

func (s *fakeSource) URL(_ context.Context, bucket, key string) (string, error) {
	s.gets++
	return "http://store/" + bucket + "/" + key, s.err
}

type fakeAnalyzer struct {
	features domain.Features
	err      error
	src      string
}

func (p *fakeAnalyzer) Analyze(_ context.Context, src string) (domain.Features, error) {
	p.src = src
	return p.features, p.err
}

type fakeRepo struct {
	stored  map[uuid.UUID]domain.Analysis
	saveErr error
	lost    bool // Save loses the race to a concurrent delivery
}

func (r *fakeRepo) Exists(_ context.Context, id uuid.UUID) (bool, error) {
	_, ok := r.stored[id]
	return ok, nil
}

func (r *fakeRepo) Save(_ context.Context, m domain.Analysis) (bool, error) {
	if r.saveErr != nil || r.lost {
		return false, r.saveErr
	}
	r.stored[m.UploadID] = m
	return true, nil
}

var song = domain.Features{
	BPM: 128, BPMConfidence: 0.71, IntegratedLUFS: -9.4, LoudnessRangeLU: 5.2, TruePeakDBTP: -0.3, SilenceRatio: 0.02, AnalyzedMs: 215000,
}

func job() mediav1.TrackUploaded {
	return mediav1.TrackUploaded{
		UploadID: uuid.NewString(), TrackID: uuid.NewString(), Bucket: "icyre-media",
		Key: "tracks/t/original/u.flac", SHA256: "ab12", UploadedAt: time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC),
	}
}

func setup() (*Service, *fakeSource, *fakeAnalyzer, *fakeRepo) {
	s, p, r := &fakeSource{}, &fakeAnalyzer{features: song}, &fakeRepo{stored: map[uuid.UUID]domain.Analysis{}}
	x := New(s, p, r, slog.New(slog.NewTextHandler(io.Discard, nil)))
	x.now = func() time.Time { return time.Date(2026, 9, 26, 10, 0, 5, 0, time.UTC) }
	return x, s, p, r
}

func TestHandleAnalyzes(t *testing.T) {
	x, _, p, r := setup()
	j := job()
	got, err := x.Handle(context.Background(), j)
	if err != nil || got != Analyzed {
		t.Fatalf("Handle = %q, %v", got, err)
	}
	if p.src != "http://store/icyre-media/tracks/t/original/u.flac" {
		t.Fatalf("analyzed %q", p.src)
	}
	m := r.stored[uuid.MustParse(j.UploadID)]
	if m.TrackID.String() != j.TrackID || m.Features != song || m.AnalyzerVersion != domain.AnalyzerVersion || m.SourceSHA256 != "ab12" || !m.UploadedAt.Equal(j.UploadedAt) || m.AnalyzedAt.IsZero() {
		t.Fatalf("stored %+v", m)
	}
}

func TestHandleRedeliveryIsNoop(t *testing.T) {
	x, s, _, _ := setup()
	j := job()
	if _, err := x.Handle(context.Background(), j); err != nil {
		t.Fatal(err)
	}
	got, err := x.Handle(context.Background(), j)
	if err != nil || got != Duplicate || s.gets != 1 {
		t.Fatalf("second Handle = %q, %v; source calls %d", got, err, s.gets)
	}
}

func TestHandleLostRaceIsDuplicate(t *testing.T) {
	x, _, _, r := setup()
	r.lost = true
	if got, err := x.Handle(context.Background(), job()); err != nil || got != Duplicate {
		t.Fatalf("Handle = %q, %v", got, err)
	}
}

func TestHandleSkipsUnusableMasters(t *testing.T) {
	cases := map[string]struct {
		srcErr   error
		features domain.Features
		err      error
		want     Outcome
	}{
		"missing":        {srcErr: ErrSourceMissing, want: Missing},
		"undecodable":    {err: errors.Join(ErrUndecodable, errors.New("bad header")), want: Undecodable},
		"invalid values": {features: domain.Features{BPM: 400, AnalyzedMs: 1000}, want: Undecodable},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			x, s, p, r := setup()
			s.err, p.err = tc.srcErr, tc.err
			if tc.features != (domain.Features{}) {
				p.features = tc.features
			}
			got, err := x.Handle(context.Background(), job())
			if err != nil || got != tc.want || len(r.stored) != 0 {
				t.Fatalf("Handle = %q, %v; stored %d", got, err, len(r.stored))
			}
		})
	}
}

func TestHandleRetryableErrors(t *testing.T) {
	boom := errors.New("boom")
	for name, arrange := range map[string]func(*fakeSource, *fakeAnalyzer, *fakeRepo){
		"storage": func(s *fakeSource, _ *fakeAnalyzer, _ *fakeRepo) { s.err = boom },
		"ffmpeg":  func(_ *fakeSource, p *fakeAnalyzer, _ *fakeRepo) { p.err = boom },
		"save":    func(_ *fakeSource, _ *fakeAnalyzer, r *fakeRepo) { r.saveErr = boom },
	} {
		t.Run(name, func(t *testing.T) {
			x, s, p, r := setup()
			arrange(s, p, r)
			if _, err := x.Handle(context.Background(), job()); !errors.Is(err, boom) {
				t.Fatalf("err = %v, want boom", err)
			}
		})
	}
}

func TestHandleRejectsInvalidJob(t *testing.T) {
	x, _, _, _ := setup()
	j := job()
	j.UploadID = "nope"
	if _, err := x.Handle(context.Background(), j); !errors.Is(err, ErrInvalidJob) {
		t.Fatalf("err = %v, want ErrInvalidJob", err)
	}
}
