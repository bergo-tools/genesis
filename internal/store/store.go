// Package store persists stories to disk. Every story lives in its own
// directory so it can be listed, backed up or deleted as a unit:
//
//	<root>/<story-id>/story.json
//	<root>/<story-id>/assets/<file>
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ErrNotFound is returned when a story (or asset) does not exist.
var ErrNotFound = errors.New("story not found")

// MaxAssetBytes caps a single uploaded image.
const MaxAssetBytes = 10 << 20

var allowedAssetExt = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
}

// Store is a directory-backed story repository.
type Store struct {
	dir   string
	mu    sync.RWMutex
	locks sync.Map // story id -> *sync.Mutex
}

// New creates the stories root directory if needed.
func New(dir string) (*Store, error) {
	if strings.TrimSpace(dir) == "" {
		dir = "stories"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

// Dir returns the stories root directory.
func (s *Store) Dir() string { return s.dir }

// TurnLock returns a per-story mutex used to serialize agent turns.
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

func validAssetName(name string) bool {
	if name == "" || strings.Contains(name, "..") || strings.ContainsAny(name, "/\\") {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

func (s *Store) storyDir(id string) string { return filepath.Join(s.dir, id) }
func (s *Store) path(id string) string     { return filepath.Join(s.storyDir(id), "story.json") }
func (s *Store) assetsDir(id string) string {
	return filepath.Join(s.storyDir(id), "assets")
}

// List returns every story, most recently updated first.
func (s *Store) List() ([]*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	out := make([]*Session, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() || !validID(e.Name()) {
			continue
		}
		b, err := os.ReadFile(s.path(e.Name()))
		if err != nil {
			continue
		}
		var sess Session
		if err := json.Unmarshal(b, &sess); err != nil {
			continue
		}
		out = append(out, &sess)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

// Get loads one story by id.
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
		return nil, fmt.Errorf("parse story %s: %w", id, err)
	}
	if sess.State == nil {
		sess.State = map[string]any{}
	}
	return &sess, nil
}

// Create assigns an id and saves a new story.
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

// Save writes story.json atomically, creating the story directory if needed.
func (s *Store) Save(sess *Session) error {
	if sess == nil || !validID(sess.ID) {
		return errors.New("store: invalid story id")
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
	if err := os.MkdirAll(s.assetsDir(sess.ID), 0o755); err != nil {
		return err
	}
	tmp := s.path(sess.ID) + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path(sess.ID))
}

// Delete removes the whole story directory.
func (s *Store) Delete(id string) error {
	if !validID(id) {
		return ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := os.Stat(s.storyDir(id)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrNotFound
		}
		return err
	}
	return os.RemoveAll(s.storyDir(id))
}

// SaveAsset stores an uploaded image inside the story directory and returns
// the generated file name.
func (s *Store) SaveAsset(id string, r io.Reader, ext string) (string, error) {
	if !validID(id) {
		return "", ErrNotFound
	}
	ext = strings.ToLower(strings.TrimSpace(ext))
	if !allowedAssetExt[ext] {
		return "", fmt.Errorf("unsupported image type %q", ext)
	}
	data, err := io.ReadAll(io.LimitReader(r, MaxAssetBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", errors.New("empty upload")
	}
	if len(data) > MaxAssetBytes {
		return "", fmt.Errorf("image too large (max %d MB)", MaxAssetBytes>>20)
	}
	name := NewID() + ext
	dir := s.assetsDir(id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		return "", err
	}
	return name, nil
}

// AssetPath returns the on-disk path of an asset, validating traversal.
func (s *Store) AssetPath(id, name string) (string, error) {
	if !validID(id) || !validAssetName(name) {
		return "", ErrNotFound
	}
	return filepath.Join(s.assetsDir(id), name), nil
}

// OpenAsset opens an asset for reading.
func (s *Store) OpenAsset(id, name string) (io.ReadCloser, error) {
	p, err := s.AssetPath(id, name)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return f, nil
}
