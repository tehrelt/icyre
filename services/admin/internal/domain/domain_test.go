package domain

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestActorPermissions(t *testing.T) {
	actorID := uuid.New()
	otherID := uuid.New()
	opOwn := Operation{ActorID: actorID}
	opOther := Operation{ActorID: otherID}

	user := Actor{ID: actorID}
	if user.CanModerateTracks() || user.CanModerateUsers() || user.CanViewAuditLogs() || user.CanViewOperation(opOwn) {
		t.Fatal("plain user must have no admin permissions")
	}

	mod := Actor{ID: actorID, Moderator: true}
	if !mod.CanModerateTracks() || mod.CanModerateUsers() || mod.CanViewAuditLogs() {
		t.Fatal("moderator permissions mismatch")
	}
	if !mod.CanViewOperation(opOwn) || mod.CanViewOperation(opOther) {
		t.Fatal("moderator should only view own operations")
	}

	admin := Actor{ID: actorID, Admin: true}
	if !admin.CanModerateTracks() || !admin.CanModerateUsers() || !admin.CanViewAuditLogs() || !admin.CanViewOperation(opOther) {
		t.Fatal("admin should have all permissions")
	}
}

func TestEnumsAndSettleImmutability(t *testing.T) {
	for _, a := range []Action{ActionTrackBlock, ActionTrackUnblock, ActionUserBlock, ActionUserUnblock} {
		if !a.Valid() {
			t.Fatalf("expected valid action %q", a)
		}
	}
	if Action("other").Valid() {
		t.Fatal("unexpected valid action")
	}
	if !TargetTrack.Valid() || !TargetUser.Valid() || TargetType("album").Valid() {
		t.Fatal("target type validation mismatch")
	}
	if StatusPending.Settled() || !StatusApplied.Settled() || !StatusNoop.Settled() || !StatusRejected.Settled() {
		t.Fatal("operation status settled mismatch")
	}
	for _, p := range []AuditPhase{PhaseRequested, PhaseApplied, PhaseNoop, PhaseRejected} {
		if !p.Valid() {
			t.Fatalf("expected valid phase %q", p)
		}
	}
	if AuditPhase("done").Valid() {
		t.Fatal("unexpected valid phase")
	}
	for _, s := range []TranscodeStatus{TranscodeQueued, TranscodeSucceeded, TranscodeFailed} {
		if !s.Valid() {
			t.Fatalf("expected valid transcode status %q", s)
		}
	}
	if TranscodeStatus("RUNNING").Valid() {
		t.Fatal("unexpected valid transcode status")
	}

	now := time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)
	orig := Operation{
		ID:          uuid.New(),
		Status:      StatusPending,
		BeforeState: map[string]any{"status": "READY"},
		AfterState:  map[string]any{},
	}
	before := map[string]any{"status": "READY"}
	after := map[string]any{"status": "BLOCKED"}
	settled := orig.Settle(StatusApplied, "APPLIED", before, after, true, now)
	before["status"] = "MUTATED"
	after["status"] = "MUTATED"

	if orig.Status != StatusPending || orig.CompletedAt != nil {
		t.Fatal("Settle mutated original operation")
	}
	if settled.Status != StatusApplied || settled.BeforeState["status"] != "READY" || settled.AfterState["status"] != "BLOCKED" || !settled.RevocationsPending {
		t.Fatalf("unexpected settled operation: %+v", settled)
	}
	if CloneMap(nil) == nil {
		t.Fatal("CloneMap(nil) must return non-nil map")
	}
}

func TestValidateModerationInputAndHash(t *testing.T) {
	id := uuid.New()
	key, reason, err := ValidateModerationInput("  key-1  ", id, "  copyright claim  ")
	if err != nil || key != "key-1" || reason != "copyright claim" {
		t.Fatalf("unexpected validation result: key=%q reason=%q err=%v", key, reason, err)
	}

	_, _, err = ValidateModerationInput("", uuid.Nil, strings.Repeat("a", 513))
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	if len(ve.Fields) != 3 {
		t.Fatalf("expected 3 invalid fields, got %+v", ve.Fields)
	}

	if _, _, err := ValidateModerationInput(strings.Repeat("k", 129), id, "ok"); err == nil {
		t.Fatal("expected error for >128 char idempotency key")
	}

	h1 := ComputeRequestHash(ActionTrackBlock, TargetTrack, id, true, "spam")
	h2 := ComputeRequestHash(ActionTrackBlock, TargetTrack, id, true, "spam")
	h3 := ComputeRequestHash(ActionTrackBlock, TargetTrack, id, false, "spam")
	if h1 != h2 || h1 == h3 || len(h1) != 64 {
		t.Fatalf("unexpected hashes: h1=%s h2=%s h3=%s", h1, h2, h3)
	}

	oe := &OwnerError{Status: 422, Code: "CANNOT_BLOCK_ADMIN", Message: "Cannot block an admin account"}
	if !strings.Contains(oe.Error(), "CANNOT_BLOCK_ADMIN") {
		t.Fatalf("unexpected OwnerError string: %s", oe.Error())
	}
}
