package store

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// MigrateLegacySessions moves records written before the preset/session split.
// Back then sessions lived in <root>/stories/<id>/story.json and carried
// messages/history. Real presets never have those fields, so a non-empty
// messages or history array marks a legacy session to relocate.
func MigrateLegacySessions(sessions *Store, stories *StoryStore) error {
	if sessions == nil || stories == nil {
		return nil
	}
	ids, err := stories.repo.ids()
	if err != nil {
		return err
	}
	for _, id := range ids {
		b, err := os.ReadFile(stories.repo.recordPath(id))
		if err != nil {
			continue
		}
		var probe struct {
			Messages []json.RawMessage `json:"messages"`
			History  []json.RawMessage `json:"history"`
		}
		if err := json.Unmarshal(b, &probe); err != nil {
			continue
		}
		if len(probe.Messages) == 0 && len(probe.History) == 0 {
			continue
		}
		dest := sessions.repo.recordDir(id)
		if _, err := os.Stat(dest); err == nil {
			continue
		}
		if err := os.MkdirAll(sessions.repo.root, 0o755); err != nil {
			return err
		}
		if err := os.Rename(stories.repo.recordDir(id), dest); err != nil {
			return err
		}
		_ = os.Rename(filepath.Join(dest, "story.json"), filepath.Join(dest, "session.json"))
	}
	return nil
}
