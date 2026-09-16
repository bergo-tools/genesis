// Package store persists roleplay sessions to disk.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ErrNotFound is returned when a session does not exist.
var ErrNotFound = errors.New("session not found")

// Store is a file-backed session repository.
type Store struct {
	dir   string
	mu    sync.RWMutex
	locks sync.Map // session id -> *sync.Mutex
}

// New creates the session directory if needed.
func New(dir string) (*Store, error) {
	if strings.TrimSpace(dir) == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

// Dir returns the directory sessions are stored in.
func (s *Store) Dir() string { return s.dir }

// TurnLock returns a per-session mutex used to serialize agent turns.
func (s *Store) TurnLock(id string) *sync.Mutex {
	v, _ := s.locks.LoadOrStore(id, &sync.Mutex{})
	return v.(*sync.Mutex)
}

func validID(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

func (s *Store) path(id string) string {
	return filepath.Join(s.dir, id+".json")
}

// List returns session summaries sorted most-recently-updated first.
func (s *Store) List() ([]*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	out := make([]*Session, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			continue
		}
		var sess Session
		if err := json.Unmarshal(b, &sess); err != nil {
			continue
		}
		out = append(out, &sess)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out, nil
}

// Get loads one session by id.
func (s *Store) Get(id string) (*Session, error) {
	if !validID(id) {
		return nil, ErrNotFound
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, err := os.ReadFile(s.path(id))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var sess Session
	if err := json.Unmarshal(b, &sess); err != nil {
		return nil, fmt.Errorf("parse session %s: %w", id, err)
	}
	if sess.State == nil {
		sess.State = map[string]any{}
	}
	return &sess, nil
}

// Create assigns an id/timestamps and saves a new session.
func (s *Store) Create(sess *Session) error {
	if sess == nil {
		return errors.New("store: nil session")
	}
	if sess.ID == "" {
		sess.ID = NewID()
	}
	now := time.Now().UTC()
	if sess.CreatedAt.IsZero() {
		sess.CreatedAt = now
	}
	sess.UpdatedAt = now
	if sess.State == nil {
		sess.State = map[string]any{}
	}
	return s.Save(sess)
}

// Save writes a session atomically.
func (s *Store) Save(sess *Session) error {
	if sess == nil || !validID(sess.ID) {
		return errors.New("store: invalid session")
	}
	sess.UpdatedAt = time.Now().UTC()
	if sess.State == nil {
		sess.State = map[string]any{}
	}
	b, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tmp := s.path(sess.ID) + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path(sess.ID))
}

// Delete removes a session.
func (s *Store) Delete(id string) error {
	if !validID(id) {
		return ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	err := os.Remove(s.path(id))
	if errors.Is(err, os.ErrNotExist) {
		return ErrNotFound
	}
	return err
}
