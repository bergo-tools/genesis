package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateLegacySessions(t *testing.T) {
	root := t.TempDir()
	stories, err := NewStoryStore(filepath.Join(root, "stories"))
	if err != nil {
		t.Fatal(err)
	}
	sessions, err := New(filepath.Join(root, "sessions"))
	if err != nil {
		t.Fatal(err)
	}

	// A legacy session: stories/<id>/story.json containing messages + history.
	legacyDir := filepath.Join(root, "stories", "old1")
	if err := os.MkdirAll(filepath.Join(legacyDir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{
		"id":       "old1",
		"title":    "Old chat",
		"messages": []map[string]any{{"id": "m1", "role": "user", "text": "hi"}},
		"history":  []map[string]any{{"role": "user", "content": "hi"}},
	})
	if err := os.WriteFile(filepath.Join(legacyDir, "story.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	// A real preset must be left alone.
	if err := stories.Create(&Story{ID: "emberfall2", Title: "Preset", Characters: []*Character{{ID: "c", Name: "N"}}}); err != nil {
		t.Fatal(err)
	}

	if err := MigrateLegacySessions(sessions, stories); err != nil {
		t.Fatal(err)
	}
	if _, err := sessions.Get("old1"); err != nil {
		t.Fatalf("legacy session was not migrated: %v", err)
	}
	if _, err := stories.Get("old1"); err != ErrNotFound {
		t.Fatalf("legacy session should be gone from presets: %v", err)
	}
	if _, err := stories.Get("emberfall2"); err != nil {
		t.Fatalf("real preset must survive migration: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "sessions", "old1", "session.json")); err != nil {
		t.Fatalf("session.json missing after migration: %v", err)
	}
}
