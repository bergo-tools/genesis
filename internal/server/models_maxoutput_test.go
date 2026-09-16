package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchChatModelsReadsMaxOutput(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[
			{"id":"x/y","name":"Y","context_length":200000,"supported_parameters":["tools"],
			 "architecture":{"output_modalities":["text"]},
			 "top_provider":{"max_completion_tokens":16000}}
		]}`)
	}))
	defer upstream.Close()
	models, err := fetchChatModels(context.Background(), upstream.URL, "k")
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 {
		t.Fatalf("want 1 model, got %d", len(models))
	}
	if models[0].MaxOutput != 16000 {
		t.Fatalf("maxOutput = %d, want 16000", models[0].MaxOutput)
	}
	if models[0].Context != 200000 {
		t.Fatalf("context = %d, want 200000", models[0].Context)
	}
}
