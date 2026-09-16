package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zp/genesis/internal/auth"
)

func do(t *testing.T, srv *Server, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

func sessionFrom(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CookieName && c.Value != "" {
			return c
		}
	}
	t.Fatal("login did not set a session cookie")
	return nil
}

func TestAuthDisabledLeavesAPIOpen(t *testing.T) {
	srv, _ := newTestServer(t)
	if rec := do(t, srv, http.MethodGet, "/api/config", "", nil); rec.Code != http.StatusOK {
		t.Fatalf("open server should serve /api/config, got %d", rec.Code)
	}
	status := do(t, srv, http.MethodGet, "/api/auth/status", "", nil)
	if !strings.Contains(status.Body.String(), `"enabled":false`) {
		t.Fatalf("status should report the gate disabled: %s", status.Body.String())
	}
}

func TestAuthGateBlocksAPIUntilLogin(t *testing.T) {
	srv, _ := newTestServerWithAuth(t, "open-sesame")

	// Static assets and the auth endpoints stay public.
	if rec := do(t, srv, http.MethodGet, "/app.js", "", nil); rec.Code != http.StatusOK {
		t.Fatalf("static assets must load before login, got %d", rec.Code)
	}
	status := do(t, srv, http.MethodGet, "/api/auth/status", "", nil)
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), `"enabled":true`) {
		t.Fatalf("status: %d %s", status.Code, status.Body.String())
	}
	if !strings.Contains(status.Body.String(), `"authenticated":false`) {
		t.Fatalf("status should start unauthenticated: %s", status.Body.String())
	}

	// Every other API call is refused without a cookie.
	blocked := do(t, srv, http.MethodGet, "/api/config", "", nil)
	if blocked.Code != http.StatusUnauthorized {
		t.Fatalf("guarded endpoint = %d, want 401", blocked.Code)
	}

	// A wrong password is rejected and issues nothing.
	if rec := do(t, srv, http.MethodPost, "/api/auth/login", `{"password":"nope"}`, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password = %d, want 401", rec.Code)
	}
	if rec := do(t, srv, http.MethodGet, "/api/config", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("still guarded after a failed login: %d", rec.Code)
	}

	// The right password returns a cookie that unlocks the API.
	login := do(t, srv, http.MethodPost, "/api/auth/login", `{"password":"open-sesame"}`, nil)
	if login.Code != http.StatusOK {
		t.Fatalf("login = %d: %s", login.Code, login.Body.String())
	}
	cookie := sessionFrom(t, login)
	if rec := do(t, srv, http.MethodGet, "/api/config", "", cookie); rec.Code != http.StatusOK {
		t.Fatalf("authenticated request = %d, want 200", rec.Code)
	}

	// Logout invalidates the same cookie.
	if rec := do(t, srv, http.MethodPost, "/api/auth/logout", "", cookie); rec.Code != http.StatusOK {
		t.Fatalf("logout = %d", rec.Code)
	}
	if rec := do(t, srv, http.MethodGet, "/api/config", "", cookie); rec.Code != http.StatusUnauthorized {
		t.Fatalf("cookie survived logout: %d", rec.Code)
	}
}

func TestAuthLoginThrottled(t *testing.T) {
	srv, _ := newTestServerWithAuth(t, "pw")
	var last *httptest.ResponseRecorder
	for i := 0; i < 6; i++ {
		last = do(t, srv, http.MethodPost, "/api/auth/login", `{"password":"bad"}`, nil)
	}
	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("expected throttling after repeated failures, got %d", last.Code)
	}
	if last.Header().Get("Retry-After") == "" {
		t.Fatal("throttled response should carry Retry-After")
	}
}
