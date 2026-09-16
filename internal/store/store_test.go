package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoryRoundTrip(t *testing.T) {
	st, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess := &Session{
		ID:         "abc123",
		Title:      "A test",
		Characters: []*Character{{ID: "c1", Name: "Ilyra"}, {ID: "c2", Name: "Bram"}},
		State:      map[string]any{"gold": 3},
		Messages:   []*Message{{ID: "m1", Role: "user", Kind: KindUser, Text: "hello"}},
	}
	if err := st.Create(sess); err != nil {
		t.Fatal(err)
	}
	got, err := st.Get("abc123")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "A test" || got.State["gold"] != float64(3) {
		t.Fatalf("unexpected round trip: %+v", got)
	}
	if names := got.CharacterNames(); len(names) != 2 || names[1] != "Bram" {
		t.Fatalf("unexpected cast: %+v", names)
	}
	list, err := st.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("list failed: %v %+v", err, list)
	}
	if c := got.FindCharacter("ilyra"); c == nil {
		t.Fatal("FindCharacter should match case-insensitively")
	}
}

func TestStoryLivesInItsOwnDirectory(t *testing.T) {
	root := t.TempDir()
	st, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Create(&Session{ID: "s1", Title: "T"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "s1", "session.json")); err != nil {
		t.Fatalf("session.json should live under its own directory: %v", err)
	}
	name, err := st.SaveAsset("s1", strings.NewReader("fake-png-bytes"), ".png")
	if err != nil {
		t.Fatal(err)
	}
	f, err := st.OpenAsset("s1", name)
	if err != nil {
		t.Fatalf("open asset: %v", err)
	}
	f.Close()
	if _, err := st.SaveAsset("s1", strings.NewReader("x"), ".svg"); err == nil {
		t.Fatal("svg uploads should be rejected")
	}
	if err := st.Delete("s1"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "s1")); !os.IsNotExist(err) {
		t.Fatal("deleting a story should remove its whole directory")
	}
}

func TestPathTraversalRejected(t *testing.T) {
	st, _ := New(t.TempDir())
	if _, err := st.Get("../secret"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for traversal id, got %v", err)
	}
	if _, err := st.OpenAsset("s1", "../story.json"); err == nil {
		t.Fatal("expected traversal asset name to be rejected")
	}
}

func TestStoryStoreIsSeparateFromSessions(t *testing.T) {
	root := t.TempDir()
	stories, err := NewStoryStore(filepath.Join(root, "stories"))
	if err != nil {
		t.Fatal(err)
	}
	sessions, err := New(filepath.Join(root, "sessions"))
	if err != nil {
		t.Fatal(err)
	}
	if err := stories.Create(&Story{
		ID:         "preset1",
		Title:      "Emberfall",
		Characters: []*Character{{ID: "c1", Name: "Ilyra"}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "stories", "preset1", "story.json")); err != nil {
		t.Fatalf("story.json missing: %v", err)
	}
	if _, err := sessions.Get("preset1"); err != ErrNotFound {
		t.Fatalf("preset must not appear as a session: %v", err)
	}
	got, err := stories.Get("preset1")
	if err != nil || got.Title != "Emberfall" {
		t.Fatalf("get story: %v %+v", err, got)
	}
	if err := stories.Delete("preset1"); err != nil {
		t.Fatal(err)
	}
	if _, err := stories.Get("preset1"); err != ErrNotFound {
		t.Fatal("expected the preset to be gone")
	}
}

func TestBuiltinStorySeededOnce(t *testing.T) {
	stories, err := NewStoryStore(filepath.Join(t.TempDir(), "stories"))
	if err != nil {
		t.Fatal(err)
	}
	if err := EnsureBuiltins(stories); err != nil {
		t.Fatal(err)
	}
	st, err := stories.Get("emberfall")
	if err != nil {
		t.Fatal(err)
	}
	if !st.Builtin || st.Title == "" || len(st.Characters) < 2 || st.Opening == "" {
		t.Fatalf("unexpected builtin preset: %+v", st)
	}
	if err := EnsureBuiltins(stories); err != nil {
		t.Fatal(err)
	}
	list, err := stories.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("expected exactly one preset after reseeding: %v %d", err, len(list))
	}
}
