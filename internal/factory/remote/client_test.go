package remote

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestClientPublishesGetsAndDownloadsLatestPack(t *testing.T) {
	packBytes := []byte("pack bytes")
	var sawAuth bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer secret" {
			sawAuth = true
		}
		switch r.URL.Path {
		case "/factory/packs":
			if r.Method != http.MethodPost {
				t.Fatalf("publish method = %s", r.Method)
			}
			if r.Header.Get("Content-Type") != "application/octet-stream" {
				t.Fatalf("publish content-type = %q", r.Header.Get("Content-Type"))
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(PackMetadata{ID: 7, PackName: "factory-core", Checksum: "sha256:abc"})
		case "/factory/packs/latest":
			_ = json.NewEncoder(w).Encode(PackMetadata{ID: 7, PackName: "factory-core", Checksum: "sha256:abc"})
		case "/factory/packs/latest/download":
			w.Write(packBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "factory-core.pack")
	if err := os.WriteFile(path, []byte("local pack bytes"), 0o600); err != nil {
		t.Fatalf("write local pack: %v", err)
	}
	client := &Client{BaseURL: server.URL + "/", Token: "secret", HTTPClient: server.Client()}
	published, err := client.PublishPack(t.Context(), path)
	if err != nil {
		t.Fatalf("PublishPack() error = %v", err)
	}
	if published.ID != 7 || published.Checksum != "sha256:abc" {
		t.Fatalf("published = %+v, want metadata", published)
	}
	latest, err := client.GetLatestPack(t.Context())
	if err != nil {
		t.Fatalf("GetLatestPack() error = %v", err)
	}
	if latest.ID != 7 {
		t.Fatalf("latest = %+v, want id 7", latest)
	}
	dest := filepath.Join(t.TempDir(), "download.pack")
	downloaded, err := client.DownloadLatestPack(t.Context(), dest)
	if err != nil {
		t.Fatalf("DownloadLatestPack() error = %v", err)
	}
	if downloaded.ID != 7 {
		t.Fatalf("download metadata = %+v, want id 7", downloaded)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read download: %v", err)
	}
	if string(data) != string(packBytes) {
		t.Fatalf("download = %q, want %q", data, packBytes)
	}
	if !sawAuth {
		t.Fatal("client did not send bearer token")
	}
}

func TestClientReturnsServerErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad token", http.StatusUnauthorized)
	}))
	defer server.Close()

	_, err := (&Client{BaseURL: server.URL, Token: "secret", HTTPClient: server.Client()}).GetLatestPack(t.Context())
	if err == nil || err.Error() != "factory server request: bad token" {
		t.Fatalf("GetLatestPack() error = %v, want server body", err)
	}
}
