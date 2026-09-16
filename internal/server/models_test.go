package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchChatModelsFiltersAndRanks(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("output_modalities"); got != "text" {
			t.Errorf("expected output_modalities=text, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[
			{"id":"z/no-tools","name":"No Tools","context_length":8000,
			 "supported_parameters":["temperature"],"architecture":{"output_modalities":["text"]}},
			{"id":"a/with-tools","name":"With Tools","context_length":128000,
			 "supported_parameters":["tools","temperature"],"architecture":{"output_modalities":["text"]}},
			{"id":"hexgrad/kokoro-82m","name":"Kokoro","supported_parameters":[],
			 "architecture":{"output_modalities":["speech"]}},
			{"id":"b/with-tools","name":"Also Tools","context_length":32000,
			 "supported_parameters":["tools"],"architecture":{"output_modalities":["text"]}}
		]}`)
	}))
	defer upstream.Close()

	models, err := fetchChatModels(context.Background(), upstream.URL, "k")
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 3 {
		t.Fatalf("speech-only model should be filtered; got %d: %+v", len(models), models)
	}
	if !models[0].Tools || !models[1].Tools {
		t.Fatalf("tool-capable models should sort first: %+v", models)
	}
	if models[0].ID != "a/with-tools" || models[1].ID != "b/with-tools" {
		t.Fatalf("unexpected order: %+v", models)
	}
	if models[2].Tools {
		t.Fatalf("last model should not support tools: %+v", models[2])
	}
	if models[0].Context != 128000 {
		t.Fatalf("context length not parsed: %+v", models[0])
	}
}

func TestFetchChatModelsProviderError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":{"message":"bad key"}}`)
	}))
	defer upstream.Close()
	if _, err := fetchChatModels(context.Background(), upstream.URL, "bad"); err == nil {
		t.Fatal("expected error for non-2xx provider response")
	}
}
