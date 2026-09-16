package llm

import "testing"

func TestNormalizeSpeechFormat(t *testing.T) {
	cases := map[string]string{
		"":      "mp3",
		"mp3":   "mp3",
		"MP3":   "mp3",
		"mpeg":  "mp3",
		"wav":   "wav",
		"wave":  "wav",
		"opus":  "opus",
		"ogg":   "opus",
		"aac":   "aac",
		"flac":  "flac",
		"pcm":   "pcm",
		"bogus": "mp3",
	}
	for in, want := range cases {
		if got := normalizeSpeechFormat(in); got != want {
			t.Fatalf("normalizeSpeechFormat(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSpeechHelpers(t *testing.T) {
	if got := SpeechExtension("wav"); got != ".wav" {
		t.Fatalf("SpeechExtension(wav) = %q", got)
	}
	if got := SpeechExtension("bogus"); got != ".mp3" {
		t.Fatalf("SpeechExtension(bogus) = %q", got)
	}
	if got := SpeechContentType("mp3"); got != "audio/mpeg" {
		t.Fatalf("SpeechContentType(mp3) = %q", got)
	}
	if got := SpeechContentType("opus"); got != "audio/ogg" {
		t.Fatalf("SpeechContentType(opus) = %q", got)
	}
}
