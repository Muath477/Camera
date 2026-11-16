package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Embedder turns an event snapshot into the 512-dimension CLIP vector the
// cloud uses for search by description. It calls the embedder container
// that runs next to Frigate (ai/embedder), so images never leave the site
// for this.
type Embedder struct {
	URL  string // http://embedder:8000/embed-image
	HTTP *http.Client
}

func NewEmbedder(url string) *Embedder {
	if url == "" {
		return nil
	}
	return &Embedder{URL: url, HTTP: &http.Client{Timeout: 10 * time.Second}}
}

// EmbedImage returns the vector for a JPEG or PNG.
func (e *Embedder) EmbedImage(ctx context.Context, img []byte) ([]float32, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.URL, bytes.NewReader(img))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "image/jpeg")
	resp, err := e.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedder: %s", resp.Status)
	}
	var out struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Embedding) != 512 {
		return nil, fmt.Errorf("embedder returned %d dimensions", len(out.Embedding))
	}
	return out.Embedding, nil
}
