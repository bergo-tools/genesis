package store

import "testing"

// Summaries must stay in step with writes without re-reading transcripts.
func TestSummariesTrackWrites(t *testing.T) {
	st, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Create(&Session{
		ID:       "s1",
		Title:    "First",
		Messages: []*Message{{ID: "m1"}, {ID: "m2"}},
	}); err != nil {
		t.Fatal(err)
	}
	list, err := st.Summaries()
	if err != nil || len(list) != 1 {
		t.Fatalf("Summaries = %v %+v", err, list)
	}
	if list[0].Title != "First" || list[0].MessageCount != 2 {
		t.Fatalf("unexpected summary: %+v", list[0])
	}

	// Untitled sessions fall back on the display name.
	if err := st.Create(&Session{ID: "s2"}); err != nil {
		t.Fatal(err)
	}
	list, _ = st.Summaries()
	if len(list) != 2 {
		t.Fatalf("want 2 summaries, got %d", len(list))
	}
	var untitled bool
	for _, s := range list {
		if s.ID == "s2" && s.Title == "Untitled" {
			untitled = true
		}
	}
	if !untitled {
		t.Fatalf("untitled session should read Untitled: %+v", list)
	}

	// A save refreshes the cached summary.
	sess, err := st.Get("s2")
	if err != nil {
		t.Fatal(err)
	}
	sess.Title = "Renamed"
	if err := st.Save(sess); err != nil {
		t.Fatal(err)
	}
	list, _ = st.Summaries()
	for _, s := range list {
		if s.ID == "s2" && s.Title != "Renamed" {
			t.Fatalf("save not reflected in summary: %+v", s)
		}
	}

	// Delete removes it.
	if err := st.Delete("s1"); err != nil {
		t.Fatal(err)
	}
	list, _ = st.Summaries()
	if len(list) != 1 || list[0].ID != "s2" {
		t.Fatalf("delete not reflected: %+v", list)
	}
}
