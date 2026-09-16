package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/zp/genesis/internal/store"
)

// A session started from a preset must own the preset's images: the browser
// resolves session avatars against the session directory.
func TestCreateSessionSnapshotsPresetAssets(t *testing.T) {
	srv, _ := newTestServer(t)
	st := &store.Story{ID: "preset", Title: "With avatar"}
	if err := srv.stories.Create(st); err != nil {
		t.Fatal(err)
	}
	name, err := srv.stories.SaveAsset("preset", strings.NewReader("png-bytes"), ".png")
	if err != nil {
		t.Fatal(err)
	}
	st.Avatar = name
	if err := srv.stories.Save(st); err != nil {
		t.Fatal(err)
	}

	rec := do(t, srv, http.MethodPost, "/api/sessions", `{"storyId":"preset"}`, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create session = %d: %s", rec.Code, rec.Body.String())
	}
	var sess store.Session
	if err := json.Unmarshal(rec.Body.Bytes(), &sess); err != nil {
		t.Fatal(err)
	}
	if sess.Avatar != name {
		t.Fatalf("session avatar = %q, want %q", sess.Avatar, name)
	}
	if !srv.store.AssetExists(sess.ID, name) {
		t.Fatal("preset avatar was not copied into the session directory")
	}
	// Deleting the preset must not remove the session's own copy.
	if err := srv.stories.Delete("preset"); err != nil {
		t.Fatal(err)
	}
	if !srv.store.AssetExists(sess.ID, name) {
		t.Fatal("session lost its avatar when the preset was deleted")
	}
}
