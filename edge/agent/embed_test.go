package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEmbedImage(t *testing.T) {
	var got []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = io.ReadAll(r.Body)
		vec := make([]float32, 512)
		vec[3] = 1
		_ = json.NewEncoder(w).Encode(map[string]any{"embedding": vec})
	}))
	defer srv.Close()

	vec, err := NewEmbedder(srv.URL).EmbedImage(context.Background(), []byte("jpeg"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "jpeg" || len(vec) != 512 || vec[3] != 1 {
		t.Errorf("sent %q, got %d dims", got, len(vec))
	}
	if NewEmbedder("") != nil {
		t.Error("an empty URL must turn the embedder off")
	}
}

func TestEmbedImageRejectsWrongSize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"embedding": []float32{1, 2, 3}})
	}))
	defer srv.Close()
	if _, err := NewEmbedder(srv.URL).EmbedImage(context.Background(), []byte("x")); err == nil {
		t.Error("a 3-dimension answer was accepted")
	}
}
