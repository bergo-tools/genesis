package server

import (
	"strings"
	"testing"

	"github.com/zp/genesis/internal/store"
)

func TestResolveVoice(t *testing.T) {
	sess := &store.Session{Characters: []*store.Character{{ID: "c1", Name: "Ilyra", Voice: "shimmer"}}}
	if got := resolveVoice(sess, "ilyra", "", "alloy"); got != "shimmer" {
		t.Fatalf("character voice not used: %q", got)
	}
	if got := resolveVoice(sess, "Ilyra", "echo", "alloy"); got != "echo" {
		t.Fatalf("explicit voice should win: %q", got)
	}
	if got := resolveVoice(sess, "Unknown", "", "alloy"); got != "alloy" {
		t.Fatalf("fallback voice not used: %q", got)
	}
	if got := resolveVoice(sess, "", "", ""); got != "" {
		t.Fatalf("expected empty voice, got %q", got)
	}
}

func TestSpeechAssetNameIsContentAddressed(t *testing.T) {
	a := speechAssetName("openai/gpt-4o-mini-tts", "alloy", "mp3", 1, "hello")
	b := speechAssetName("openai/gpt-4o-mini-tts", "alloy", "mp3", 1, "hello")
	c := speechAssetName("openai/gpt-4o-mini-tts", "echo", "mp3", 1, "hello")
	d := speechAssetName("openai/gpt-4o-mini-tts", "alloy", "mp3", 1.5, "hello")
	if a != b {
		t.Fatal("identical input must produce the same cached name")
	}
	if a == c {
		t.Fatal("a different voice must produce a different cached name")
	}
	if a == d {
		t.Fatal("a different speed must produce a different cached name")
	}
	if !strings.HasPrefix(a, "speech-") {
		t.Fatalf("unexpected asset name %q", a)
	}
}

func TestFindMessage(t *testing.T) {
	sess := &store.Session{Messages: []*store.Message{{ID: "m1", Text: "hi"}}}
	if m := findMessage(sess, "m1"); m == nil || m.Text != "hi" {
		t.Fatal("expected to find the message")
	}
	if m := findMessage(sess, "nope"); m != nil {
		t.Fatal("expected nil for unknown id")
	}
}
