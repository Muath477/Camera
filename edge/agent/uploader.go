package agent

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Uploader asks the device API for a presigned URL and uploads media straight
// to object storage, so large bytes never pass through the cloud services.
type Uploader struct {
	api  string
	http *http.Client
}

func NewUploader(api string, tc *tls.Config) *Uploader {
	return &Uploader{
		api:  api,
		http: &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{TLSClientConfig: tc}},
	}
}

func (u *Uploader) Enabled() bool { return u.api != "" }

// Upload returns the object key the media was stored under.
func (u *Uploader) Upload(ctx context.Context, kind, camera, contentType string, body []byte) (string, error) {
	reqBody, _ := json.Marshal(map[string]any{"kind": kind, "camera": camera, "content_type": contentType, "bytes": len(body)})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.api+"/device/v1/uploads", bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := u.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("upload slot: %s", resp.Status)
	}
	var slot struct {
		Key string `json:"key"`
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&slot); err != nil {
		return "", err
	}
	if err := putObject(ctx, slot.URL, contentType, body); err != nil {
		return "", err
	}
	return slot.Key, nil
}
