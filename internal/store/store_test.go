package store

import "testing"

func TestSessionRoundTrip(t *testing.T) {
	st, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess := &Session{
		ID:         "abc123",
		Title:      "A test",
		Characters: []*Character{{ID: "c1", Name: "Ilyra"}},
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
	list, err := st.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("list failed: %v %+v", err, list)
	}
	if c := got.FindCharacter("ilyra"); c == nil {
		t.Fatal("FindCharacter should match case-insensitively")
	}
	if err := st.Delete("abc123"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Get("abc123"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestValidIDRejectsTraversal(t *testing.T) {
	st, _ := New(t.TempDir())
	if _, err := st.Get("../secret"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for traversal id, got %v", err)
	}
}
