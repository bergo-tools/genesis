package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchSpeechModelsReadsVoiceCatalog(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("output_modalities"); got != "speech" {
			t.Errorf("expected output_modalities=speech, got %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("expected bearer auth, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[
			{"id":"hexgrad/kokoro-82m","name":"Kokoro","supported_voices":["af_heart","am_adam","zf_xiaoxiao"]},
			{"id":"deepgram/flux-tts:free","name":"Flux","supported_voices":["flux-alexis-en"]},
			{"id":"no/voices","name":"No voices"}
		]}`)
	}))
	defer upstream.Close()

	models, err := fetchSpeechModels(context.Background(), upstream.URL, "test-key")
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 3 {
		t.Fatalf("want 3 models, got %d", len(models))
	}
	if got := models[0].Voices; len(got) != 3 || got[0] != "af_heart" {
		t.Fatalf("unexpected voices: %+v", got)
	}
	if models[2].Voices != nil {
		t.Fatalf("models without a catalog should have nil voices, got %+v", models[2].Voices)
	}
	if models[1].ID != "deepgram/flux-tts:free" {
		t.Fatalf("id with suffix lost: %q", models[1].ID)
	}
}

func TestFetchSpeechModelsSurfacesProviderErrors(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":{"message":"bad key"}}`)
	}))
	defer upstream.Close()
	if _, err := fetchSpeechModels(context.Background(), upstream.URL, "bad"); err == nil {
		t.Fatal("expected an error for a non-2xx provider response")
	}
}
