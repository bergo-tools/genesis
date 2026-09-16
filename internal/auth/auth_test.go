package auth

import (
	"testing"
	"time"
)

func TestDisabledWithoutPassword(t *testing.T) {
	a := New("")
	if a.Enabled() {
		t.Fatal("empty password should disable the gate")
	}
	if a.Valid("anything") {
		t.Fatal("no token can be valid while disabled")
	}
	if att := a.Login("", "1.2.3.4"); att.OK {
		t.Fatal("login should fail while disabled")
	}
}

func TestLoginIssuesAndValidatesToken(t *testing.T) {
	a := New("hunter2")
	if !a.Enabled() {
		t.Fatal("expected the gate to be enabled")
	}
	if att := a.Login("wrong", "ip"); att.OK {
		t.Fatal("wrong password must not issue a token")
	}
	att := a.Login("hunter2", "ip")
	if !att.OK || att.Token == "" {
		t.Fatalf("correct password rejected: %+v", att)
	}
	if !a.Valid(att.Token) {
		t.Fatal("issued token should be valid")
	}
	if a.Valid("not-a-token") {
		t.Fatal("bogus token must not be valid")
	}
	a.Logout(att.Token)
	if a.Valid(att.Token) {
		t.Fatal("token should die on logout")
	}
}

func TestSessionsExpire(t *testing.T) {
	a := New("pw")
	base := time.Now()
	a.now = func() time.Time { return base }
	att := a.Login("pw", "ip")
	if !att.OK {
		t.Fatal("login failed")
	}
	a.now = func() time.Time { return base.Add(DefaultTTL + time.Minute) }
	if a.Valid(att.Token) {
		t.Fatal("expired token should be rejected")
	}
	if a.Active() != 0 {
		t.Fatalf("expired session not reaped: %d live", a.Active())
	}
}

func TestActiveUseSlidesExpiry(t *testing.T) {
	a := New("pw")
	base := time.Now()
	a.now = func() time.Time { return base }
	att := a.Login("pw", "ip")
	// Half-way through the TTL, touching the session should extend it.
	a.now = func() time.Time { return base.Add(DefaultTTL / 2) }
	if !a.Valid(att.Token) {
		t.Fatal("token should still be valid")
	}
	a.now = func() time.Time { return base.Add(DefaultTTL - time.Minute) }
	if !a.Valid(att.Token) {
		t.Fatal("sliding expiry should keep an active session alive")
	}
}

func TestThrottlesRepeatedFailures(t *testing.T) {
	a := New("pw")
	base := time.Now()
	a.now = func() time.Time { return base }
	for i := 0; i < maxAttempts; i++ {
		if att := a.Login("nope", "ip"); att.OK {
			t.Fatal("wrong password accepted")
		}
	}
	att := a.Login("pw", "ip")
	if att.OK {
		t.Fatal("locked-out client must be refused even with the right password")
	}
	if att.RetryAfter <= 0 {
		t.Fatalf("expected a retry hint, got %+v", att)
	}
	// Another client is unaffected.
	if other := a.Login("pw", "other"); !other.OK {
		t.Fatal("throttling one client must not block another")
	}
	a.now = func() time.Time { return base.Add(lockout + time.Second) }
	if again := a.Login("pw", "ip"); !again.OK {
		t.Fatal("cooldown should expire")
	}
}
