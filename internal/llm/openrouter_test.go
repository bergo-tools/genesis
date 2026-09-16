package llm

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalizeEffort(t *testing.T) {
	cases := map[string]string{
		"":         "",
		"default":  "",
		"provider": "",
		"off":      "none",
		"none":     "none",
		"disabled": "none",
		"minimal":  "minimal",
		"low":      "low",
		"medium":   "medium",
		"high":     "high",
		"max":      "max",
		"xhigh":    "xhigh",
		"MEDIUM":   "medium",
		"nonsense": "",
	}
	for in, want := range cases {
		if got := normalizeEffort(in); got != want {
			t.Fatalf("normalizeEffort(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUserMessageWithImagesIsMultimodal(t *testing.T) {
	msg := toSDKUserMessage(Message{
		Role:    RoleUser,
		Content: "look",
		Images:  []Image{{Name: "a.png", DataURL: "data:image/png;base64,AAAA"}},
	})
	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	if !strings.Contains(got, `"image_url"`) || !strings.Contains(got, "data:image/png") {
		t.Fatalf("expected multimodal content, got %s", got)
	}
	if !strings.Contains(got, `"text":"look"`) {
		t.Fatalf("expected a text part, got %s", got)
	}
}

func TestUserMessageWithoutImagesStaysAString(t *testing.T) {
	msg := toSDKUserMessage(Message{Role: RoleUser, Content: "hi"})
	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "image_url") {
		t.Fatalf("did not expect multimodal content: %s", string(b))
	}
	if !strings.Contains(string(b), `"content":"hi"`) {
		t.Fatalf("unexpected content: %s", string(b))
	}
}
