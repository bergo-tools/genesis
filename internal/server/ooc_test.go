package server

import "testing"

func TestUserTurnContent(t *testing.T) {
	cases := []struct {
		text, ooc, want string
	}{
		{"hello", "", "hello"},
		{"", "be brief", "[OOC] be brief"},
		{"hello", "be brief", "hello\n\n[OOC] be brief"},
		{"  hi  ", "  x  ", "hi\n\n[OOC] x"},
		{"", "", ""},
	}
	for _, c := range cases {
		if got := userTurnContent(c.text, c.ooc); got != c.want {
			t.Fatalf("userTurnContent(%q, %q) = %q, want %q", c.text, c.ooc, got, c.want)
		}
	}
}
