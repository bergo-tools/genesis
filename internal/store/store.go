// Package store persists presets (stories) and their conversation instances
// (sessions) to disk. Each record lives in its own directory so it can be
// listed, backed up or deleted as a unit:
//
//	<dataDir>/stories/<story-id>/story.json + assets/
//	<dataDir>/sessions/<session-id>/session.json + assets/
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

// ErrNotFound is returned when a record (or asset) does not exist.
var ErrNotFound = errors.New("not found")

// MaxAssetBytes caps a single uploaded image.
const MaxAssetBytes = 10 << 20

var allowedAssetExt = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
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

// repo is a directory-per-record collection.
type repo struct {
	root     string
	fileName string
	mu       sync.RWMutex
	locks    sync.Map
}

func newRepo(root, fileName string) (*repo, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("store: empty root")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return &repo{root: root, fileName: fileName}, nil
}

func (r *repo) recordDir(id string) string  { return filepath.Join(r.root, id) }
func (r *repo) recordPath(id string) string { return filepath.Join(r.recordDir(id), r.fileName) }
func (r *repo) assetsDir(id string) string  { return filepath.Join(r.recordDir(id), "assets") }

func (r *repo) turnLock(id string) *sync.Mutex {
	v, _ := r.locks.LoadOrStore(id, &sync.Mutex{})
	return v.(*sync.Mutex)
}

func (r *repo) save(id string, v any) error {
	if !validID(id) {
		return errors.New("store: invalid id")
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := os.MkdirAll(r.assetsDir(id), 0o755); err != nil {
		return err
	}
	tmp := r.recordPath(id) + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, r.recordPath(id))
}

func (r *repo) load(id string, v any) error {
	if !validID(id) {
		return ErrNotFound
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	b, err := os.ReadFile(r.recordPath(id))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrNotFound
		}
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("parse %s: %w", id, err)
	}
	return nil
}

func (r *repo) ids() ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entries, err := os.ReadDir(r.root)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() && validID(e.Name()) {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

func (r *repo) remove(id string) error {
	if !validID(id) {
		return ErrNotFound
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, err := os.Stat(r.recordDir(id)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrNotFound
		}
		return err
	}
	return os.RemoveAll(r.recordDir(id))
}

func (r *repo) assetPath(id, name string) (string, error) {
	if !validID(id) || !validAssetName(name) {
		return "", ErrNotFound
	}
	return filepath.Join(r.assetsDir(id), name), nil
}

func (r *repo) openAsset(id, name string) (io.ReadCloser, error) {
	p, err := r.assetPath(id, name)
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

func (r *repo) assetExists(id, name string) bool {
	p, err := r.assetPath(id, name)
	if err != nil {
		return false
	}
	_, statErr := os.Stat(p)
	return statErr == nil
}

func (r *repo) writeAsset(id, name string, data []byte) error {
	p, err := r.assetPath(id, name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(r.assetsDir(id), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

func (r *repo) saveUpload(id string, rd io.Reader, ext string) (string, error) {
	if !validID(id) {
		return "", ErrNotFound
	}
	ext = strings.ToLower(strings.TrimSpace(ext))
	if !allowedAssetExt[ext] {
		return "", fmt.Errorf("unsupported image type %q", ext)
	}
	data, err := io.ReadAll(io.LimitReader(rd, MaxAssetBytes+1))
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
	if err := r.writeAsset(id, name, data); err != nil {
		return "", err
	}
	return name, nil
}

// ---------------------------------------------------------------- sessions

// SessionSummary is the lightweight record the sidebar needs. Listing sessions
// must not parse every transcript, so the store keeps summaries in memory and
// updates them on every write.
type SessionSummary struct {
	ID           string    `json:"id"`
	StoryID      string    `json:"storyId,omitempty"`
	StoryTitle   string    `json:"storyTitle,omitempty"`
	Title        string    `json:"title"`
	Avatar       string    `json:"avatar,omitempty"`
	Characters   []string  `json:"characters"`
	MessageCount int       `json:"messageCount"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// Store is the session (conversation) repository.
type Store struct {
	repo *repo

	mu        sync.RWMutex
	summaries map[string]*SessionSummary
}

// New opens the session repository rooted at dir.
func New(dir string) (*Store, error) {
	r, err := newRepo(dir, "session.json")
	if err != nil {
		return nil, err
	}
	return &Store{repo: r, summaries: map[string]*SessionSummary{}}, nil
}

// Summaries lists sessions for the sidebar. It reads the directory (cheap) and
// only parses sessions it has not seen, so deleting a session directory by hand
// still makes it disappear without re-reading every transcript.
func (s *Store) Summaries() ([]*SessionSummary, error) {
	ids, err := s.repo.ids()
	if err != nil {
		return nil, err
	}
	present := make(map[string]bool, len(ids))
	for _, id := range ids {
		present[id] = true
	}

	s.mu.Lock()
	if s.summaries == nil {
		s.summaries = map[string]*SessionSummary{}
	}
	// Forget sessions whose directory is gone.
	for id := range s.summaries {
		if !present[id] {
			delete(s.summaries, id)
		}
	}
	out := make([]*SessionSummary, 0, len(ids))
	stale := make([]string, 0, len(ids))
	for _, id := range ids {
		if sum, ok := s.summaries[id]; ok {
			out = append(out, sum.clone())
		} else {
			stale = append(stale, id)
		}
	}
	s.mu.Unlock()

	for _, id := range stale {
		var sess Session
		if err := s.repo.load(id, &sess); err != nil {
			continue
		}
		sum := summarize(&sess)
		if sum == nil {
			continue
		}
		s.mu.Lock()
		s.summaries[id] = sum
		s.mu.Unlock()
		out = append(out, sum.clone())
	}

	sortSummaries(out)
	return out, nil
}

func (s *Store) cacheSummary(sess *Session) {
	if sess == nil {
		return
	}
	s.mu.Lock()
	if s.summaries == nil {
		s.summaries = map[string]*SessionSummary{}
	}
	s.summaries[sess.ID] = summarize(sess)
	s.mu.Unlock()
}

func (s *Store) forgetSummary(id string) {
	s.mu.Lock()
	delete(s.summaries, id)
	s.mu.Unlock()
}

func summarize(sess *Session) *SessionSummary {
	if sess == nil {
		return nil
	}
	title := strings.TrimSpace(sess.Title)
	if title == "" {
		title = "Untitled"
	}
	return &SessionSummary{
		ID:           sess.ID,
		StoryID:      sess.StoryID,
		StoryTitle:   sess.StoryTitle,
		Title:        title,
		Avatar:       sess.Avatar,
		Characters:   sess.CharacterNames(),
		MessageCount: len(sess.Messages),
		CreatedAt:    sess.CreatedAt,
		UpdatedAt:    sess.UpdatedAt,
	}
}

func (v *SessionSummary) clone() *SessionSummary {
	if v == nil {
		return nil
	}
	cp := *v
	if v.Characters != nil {
		cp.Characters = append([]string(nil), v.Characters...)
	}
	return &cp
}

func sortSummaries(list []*SessionSummary) {
	sort.Slice(list, func(i, j int) bool { return list[i].UpdatedAt.After(list[j].UpdatedAt) })
}

// Dir returns the sessions root directory.
func (s *Store) Dir() string { return s.repo.root }

// TurnLock serializes agent turns per session.
func (s *Store) TurnLock(id string) *sync.Mutex { return s.repo.turnLock(id) }

// List returns every session, most recently updated first.
func (s *Store) List() ([]*Session, error) {
	ids, err := s.repo.ids()
	if err != nil {
		return nil, err
	}
	out := make([]*Session, 0, len(ids))
	for _, id := range ids {
		var sess Session
		if err := s.repo.load(id, &sess); err != nil {
			continue
		}
		out = append(out, &sess)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

// Get loads one session by id.
func (s *Store) Get(id string) (*Session, error) {
	var sess Session
	if err := s.repo.load(id, &sess); err != nil {
		return nil, err
	}
	if sess.State == nil {
		sess.State = map[string]any{}
	}
	return &sess, nil
}

// Create assigns an id and saves a new session.
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
	if err := s.repo.save(sess.ID, sess); err != nil {
		return err
	}
	s.cacheSummary(sess)
	return nil
}

// Save writes a session.
func (s *Store) Save(sess *Session) error {
	if sess == nil {
		return errors.New("store: nil session")
	}
	sess.UpdatedAt = time.Now().UTC()
	if sess.State == nil {
		sess.State = map[string]any{}
	}
	if err := s.repo.save(sess.ID, sess); err != nil {
		return err
	}
	s.cacheSummary(sess)
	return nil
}

// Delete removes a session and all of its assets.
func (s *Store) Delete(id string) error {
	if err := s.repo.remove(id); err != nil {
		return err
	}
	s.forgetSummary(id)
	return nil
}

// SaveAsset stores an uploaded image inside the session directory.
func (s *Store) SaveAsset(id string, rd io.Reader, ext string) (string, error) {
	return s.repo.saveUpload(id, rd, ext)
}

// AssetPath returns the on-disk path of a session asset.
func (s *Store) AssetPath(id, name string) (string, error) { return s.repo.assetPath(id, name) }

// OpenAsset opens a session asset.
func (s *Store) OpenAsset(id, name string) (io.ReadCloser, error) { return s.repo.openAsset(id, name) }

// AssetExists reports whether a session asset exists.
func (s *Store) AssetExists(id, name string) bool { return s.repo.assetExists(id, name) }

// WriteAsset stores bytes under an explicit name.
func (s *Store) WriteAsset(id, name string, data []byte) error {
	return s.repo.writeAsset(id, name, data)
}

// ----------------------------------------------------------------- stories

// StoryStore is the story (preset) repository.
type StoryStore struct {
	repo *repo
}

// NewStoryStore opens the preset repository rooted at dir.
func NewStoryStore(dir string) (*StoryStore, error) {
	r, err := newRepo(dir, "story.json")
	if err != nil {
		return nil, err
	}
	return &StoryStore{repo: r}, nil
}

// Dir returns the presets root directory.
func (s *StoryStore) Dir() string { return s.repo.root }

// List returns every preset, most recently updated first.
func (s *StoryStore) List() ([]*Story, error) {
	ids, err := s.repo.ids()
	if err != nil {
		return nil, err
	}
	out := make([]*Story, 0, len(ids))
	for _, id := range ids {
		var st Story
		if err := s.repo.load(id, &st); err != nil {
			continue
		}
		out = append(out, &st)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

// Get loads one preset by id.
func (s *StoryStore) Get(id string) (*Story, error) {
	var st Story
	if err := s.repo.load(id, &st); err != nil {
		return nil, err
	}
	if st.State == nil {
		st.State = map[string]any{}
	}
	return &st, nil
}

// Create assigns an id and saves a new preset.
func (s *StoryStore) Create(st *Story) error {
	if st == nil {
		return errors.New("store: nil story")
	}
	if st.ID == "" {
		st.ID = NewID()
	}
	now := time.Now().UTC()
	if st.CreatedAt.IsZero() {
		st.CreatedAt = now
	}
	st.UpdatedAt = now
	if st.State == nil {
		st.State = map[string]any{}
	}
	return s.repo.save(st.ID, st)
}

// Save writes a preset.
func (s *StoryStore) Save(st *Story) error {
	if st == nil {
		return errors.New("store: nil story")
	}
	st.UpdatedAt = time.Now().UTC()
	if st.State == nil {
		st.State = map[string]any{}
	}
	return s.repo.save(st.ID, st)
}

// Delete removes a preset and all of its assets.
func (s *StoryStore) Delete(id string) error { return s.repo.remove(id) }

// SaveAsset stores an uploaded image inside the preset directory.
func (s *StoryStore) SaveAsset(id string, rd io.Reader, ext string) (string, error) {
	return s.repo.saveUpload(id, rd, ext)
}

// AssetPath returns the on-disk path of a preset asset.
func (s *StoryStore) AssetPath(id, name string) (string, error) { return s.repo.assetPath(id, name) }

// OpenAsset opens a preset asset.
func (s *StoryStore) OpenAsset(id, name string) (io.ReadCloser, error) {
	return s.repo.openAsset(id, name)
}
