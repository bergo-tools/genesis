package server

import (
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/zp/genesis/internal/auth"
)

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	enabled := s.auth.Enabled()
	authed := !enabled || s.auth.Valid(sessionToken(r))
	writeJSON(w, http.StatusOK, map[string]any{"enabled": enabled, "authenticated": authed})
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	attempt := s.auth.Login(body.Password, clientIP(r))
	switch {
	case attempt.OK:
		http.SetCookie(w, sessionCookie(r, attempt.Token, int(s.auth.TTL().Seconds())))
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case attempt.RetryAfter > 0:
		w.Header().Set("Retry-After", strconv.Itoa(int(attempt.RetryAfter.Seconds())+1))
		writeError(w, http.StatusTooManyRequests, errors.New("too many failed attempts; wait a moment and try again"))
	default:
		writeError(w, http.StatusUnauthorized, errors.New("wrong password"))
	}
}

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	s.auth.Logout(sessionToken(r))
	http.SetCookie(w, sessionCookie(r, "", -1))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// requireAuth rejects API calls without a live session. Static files stay open
// so the login screen can load; the auth and health endpoints answer too.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	if !s.auth.Enabled() {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") || publicAPIPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		if s.auth.Valid(sessionToken(r)) {
			next.ServeHTTP(w, r)
			return
		}
		writeError(w, http.StatusUnauthorized, errors.New("authentication required"))
	})
}

// publicAPIPath lists endpoints that must answer before a session exists.
func publicAPIPath(path string) bool {
	return path == "/api/health" || strings.HasPrefix(path, "/api/auth/")
}

func sessionToken(r *http.Request) string {
	c, err := r.Cookie(auth.CookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

func sessionCookie(r *http.Request, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     auth.CookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	}
}

// clientIP identifies a caller for throttling failures. Genesis usually listens
// on localhost, so the socket address is the honest answer.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
