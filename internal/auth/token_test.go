package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ebnsina/saydalah-api/internal/store"
)

func TestTokenRoundTrip(t *testing.T) {
	tm := NewTokenManager("test-secret", time.Hour)
	branch := uuid.New()
	want := Identity{UserID: uuid.New(), Role: store.UserRolePharmacist, BranchID: &branch}

	token, expiresAt, err := tm.Issue(want, time.Now())
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if !expiresAt.After(time.Now()) {
		t.Errorf("expiry should be in the future")
	}

	got, err := tm.Parse(token)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.UserID != want.UserID || got.Role != want.Role ||
		got.BranchID == nil || *got.BranchID != branch {
		t.Errorf("round-trip mismatch: got %+v want %+v", got, want)
	}
}

func TestParseRejectsExpiredToken(t *testing.T) {
	tm := NewTokenManager("test-secret", time.Hour)
	token, _, err := tm.Issue(Identity{UserID: uuid.New(), Role: store.UserRoleAdmin}, time.Now().Add(-2*time.Hour))
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if _, err := tm.Parse(token); err == nil {
		t.Error("expected expired token to be rejected")
	}
}

func TestParseRejectsWrongSecret(t *testing.T) {
	issuer := NewTokenManager("secret-a", time.Hour)
	verifier := NewTokenManager("secret-b", time.Hour)
	token, _, _ := issuer.Issue(Identity{UserID: uuid.New(), Role: store.UserRoleAdmin}, time.Now())
	if _, err := verifier.Parse(token); err == nil {
		t.Error("expected token signed with a different secret to be rejected")
	}
}

func TestPasswordHashing(t *testing.T) {
	hash, err := HashPassword("s3cret-pass")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !CheckPassword(hash, "s3cret-pass") {
		t.Error("correct password should verify")
	}
	if CheckPassword(hash, "wrong") {
		t.Error("wrong password must not verify")
	}
}

func TestResolveBranch(t *testing.T) {
	branchA := uuid.New()
	branchB := uuid.New()
	admin := Identity{Role: store.UserRoleAdmin}
	staff := Identity{Role: store.UserRoleCashier, BranchID: &branchA}

	// Admin must name a branch.
	if _, err := admin.ResolveBranch(nil); err == nil {
		t.Error("admin without branch should error")
	}
	if got, err := admin.ResolveBranch(&branchB); err != nil || got != branchB {
		t.Errorf("admin should resolve to requested branch: %v %v", got, err)
	}
	// Staff pinned to their own branch.
	if got, err := staff.ResolveBranch(nil); err != nil || got != branchA {
		t.Errorf("staff should resolve to own branch: %v %v", got, err)
	}
	if _, err := staff.ResolveBranch(&branchB); err == nil {
		t.Error("staff requesting another branch should be forbidden")
	}
}
