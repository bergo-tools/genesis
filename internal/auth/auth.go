// Package auth gates Genesis behind a single shared password. The password
// comes from the environment, and issued sessions live only in memory, so
// restarting the process signs everyone out.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"strings"
	"sync"
	"time"
)

// CookieName is the cookie that carries a session token to the browser.
const CookieName = "genesis_session"

// DefaultTTL is how long a session survives between visits.
const DefaultTTL = 30 * 24 * time.Hour

// maxAttempts is how many wrong passwords one client may try in a row.
const maxAttempts = 5

// lockout is how long a client waits after too many failures.
const lockout = 30 * time.Second

// Attempt reports the outcome of a login.
type Attempt struct {
	OK         bool
	Token      string
	RetryAfter time.Duration
}

// Authenticator compares passwords and tracks live sessions in memory.
type Authenticator struct {
	password string
	ttl      time.Duration

	mu       sync.Mutex
	sessions map[string]time.Time
	failures map[string]*failure
	now      func() time.Time
}

type failure struct {
	count   int
	blocked time.Time
}

// New builds an Authenticator. An empty password disables the gate entirely.
func New(password string) *Authenticator {
	return &Authenticator{
		password: password,
		ttl:      DefaultTTL,
		sessions: map[string]time.Time{},
		failures: map[string]*failure{},
		now:      time.Now,
	}
}

// Enabled reports whether a password is configured.
func (a *Authenticator) Enabled() bool {
	return a != nil && strings.TrimSpace(a.password) != ""
}

// TTL returns the idle lifetime of a session.
func (a *Authenticator) TTL() time.Duration {
	if a == nil || a.ttl <= 0 {
		return DefaultTTL
	}
	return a.ttl
}

// Login checks a password and, on success, issues a fresh session token.
func (a *Authenticator) Login(password, client string) Attempt {
	if !a.Enabled() {
		return Attempt{}
	}
	now := a.now()
	a.mu.Lock()
	defer a.mu.Unlock()
	if f := a.failures[client]; f != nil && now.Before(f.blocked) {
		return Attempt{RetryAfter: f.blocked.Sub(now)}
	}
	if !equal(a.password, password) {
		f := a.failures[client]
		if f == nil {
			f = &failure{}
			a.failures[client] = f
		}
		f.count++
		if f.count >= maxAttempts {
			f.count = 0
			f.blocked = now.Add(lockout)
		}
		a.pruneLocked(now)
		return Attempt{}
	}
	delete(a.failures, client)
	token, err := newToken()
	if err != nil {
		return Attempt{}
	}
	a.sessions[token] = now.Add(a.TTL())
	return Attempt{OK: true, Token: token}
}

// Valid reports whether a token names a live session, sliding its expiry on
// each visit so an active browser is not signed out mid-story.
func (a *Authenticator) Valid(token string) bool {
	if !a.Enabled() || token == "" {
		return false
	}
	now := a.now()
	a.mu.Lock()
	defer a.mu.Unlock()
	expires, ok := a.sessions[token]
	if !ok {
		return false
	}
	if !now.Before(expires) {
		delete(a.sessions, token)
		return false
	}
	a.sessions[token] = now.Add(a.TTL())
	return true
}

// Logout forgets a session.
func (a *Authenticator) Logout(token string) {
	if a == nil || token == "" {
		return
	}
	a.mu.Lock()
	delete(a.sessions, token)
	a.mu.Unlock()
}

// Active reports how many sessions are live, for logging and tests.
func (a *Authenticator) Active() int {
	if a == nil {
		return 0
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.sessions)
}

// pruneLocked drops stale failure records once the table grows. The caller
// holds a.mu.
func (a *Authenticator) pruneLocked(now time.Time) {
	if len(a.failures) <= 1024 {
		return
	}
	for client, f := range a.failures {
		if f.count == 0 && now.After(f.blocked) {
			delete(a.failures, client)
		}
	}
}

func newToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// equal compares two secrets without leaking their contents through timing.
func equal(want, got string) bool {
	return subtle.ConstantTimeCompare([]byte(want), []byte(got)) == 1
}
