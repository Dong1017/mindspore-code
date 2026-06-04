package server

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/mindspore-lab/mindspore-cli/configs"
	"github.com/mindspore-lab/mindspore-cli/internal/factory/compiler"
	"github.com/mindspore-lab/mindspore-cli/internal/factory/pack"
)

func TestFactoryPackRoutesPublishLatestAndDownload(t *testing.T) {
	store := newTestStore(t)
	mux := NewMux(store, []configs.TokenEntry{{Token: "secret", User: "alice", Role: "developer"}}, nil)
	packPath := buildTestFactoryPack(t)
	body := readTestFile(t, packPath)

	publish := httptest.NewRequest(http.MethodPost, "/factory/packs", bytes.NewReader(body))
	publish.Header.Set("Authorization", "Bearer secret")
	publish.Header.Set("Content-Type", "application/octet-stream")
	publishRec := httptest.NewRecorder()
	mux.ServeHTTP(publishRec, publish)
	if publishRec.Code != http.StatusCreated {
		t.Fatalf("publish status = %d body = %s", publishRec.Code, publishRec.Body.String())
	}
	var metadata factoryPackMetadataResponse
	if err := json.Unmarshal(publishRec.Body.Bytes(), &metadata); err != nil {
		t.Fatalf("decode publish metadata: %v", err)
	}
	if metadata.PackName != "factory-core" || metadata.Publisher != "alice" || metadata.CompiledCaseCount != 3 || metadata.Checksum == "" {
		t.Fatalf("metadata = %+v, want factory-core alice with checksum", metadata)
	}

	latest := httptest.NewRequest(http.MethodGet, "/factory/packs/latest", nil)
	latest.Header.Set("Authorization", "Bearer secret")
	latestRec := httptest.NewRecorder()
	mux.ServeHTTP(latestRec, latest)
	assertStatus(t, latestRec, http.StatusOK)

	idNoAuth := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/factory/packs/%d/download", metadata.ID), nil)
	idNoAuthRec := httptest.NewRecorder()
	mux.ServeHTTP(idNoAuthRec, idNoAuth)
	assertStatus(t, idNoAuthRec, http.StatusUnauthorized)

	idDownload := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/factory/packs/%d/download", metadata.ID), nil)
	idDownload.Header.Set("Authorization", "Bearer secret")
	idDownloadRec := httptest.NewRecorder()
	mux.ServeHTTP(idDownloadRec, idDownload)
	assertStatus(t, idDownloadRec, http.StatusOK)
	if !bytes.Equal(idDownloadRec.Body.Bytes(), body) {
		t.Fatalf("id download bytes differ from uploaded pack")
	}
	idDownPath := filepath.Join(t.TempDir(), pack.FileName)
	writeTestFile(t, idDownPath, idDownloadRec.Body.Bytes())
	if _, err := pack.Load(idDownPath); err != nil {
		t.Fatalf("load id downloaded pack: %v", err)
	}

	download := httptest.NewRequest(http.MethodGet, "/factory/packs/latest/download", nil)
	download.Header.Set("Authorization", "Bearer secret")
	downloadRec := httptest.NewRecorder()
	mux.ServeHTTP(downloadRec, download)
	assertStatus(t, downloadRec, http.StatusOK)
	if downloadRec.Header().Get("Content-Type") != "application/octet-stream" {
		t.Fatalf("download content-type = %q", downloadRec.Header().Get("Content-Type"))
	}
	downPath := filepath.Join(t.TempDir(), pack.FileName)
	writeTestFile(t, downPath, downloadRec.Body.Bytes())
	loaded, err := pack.Load(downPath)
	if err != nil {
		t.Fatalf("load downloaded pack: %v", err)
	}
	if loaded.Manifest.Checksum != metadata.Checksum {
		t.Fatalf("downloaded checksum = %q, want %q", loaded.Manifest.Checksum, metadata.Checksum)
	}
}

func TestFactoryPackRoutesRequireAuthAndValidatePack(t *testing.T) {
	store := newTestStore(t)
	mux := NewMux(store, []configs.TokenEntry{{Token: "secret", User: "alice", Role: "developer"}}, nil)

	noAuth := httptest.NewRequest(http.MethodGet, "/factory/packs/latest", nil)
	noAuthRec := httptest.NewRecorder()
	mux.ServeHTTP(noAuthRec, noAuth)
	assertStatus(t, noAuthRec, http.StatusUnauthorized)

	invalid := httptest.NewRequest(http.MethodPost, "/factory/packs", bytes.NewReader([]byte("not a pack")))
	invalid.Header.Set("Authorization", "Bearer secret")
	invalidRec := httptest.NewRecorder()
	mux.ServeHTTP(invalidRec, invalid)
	assertStatus(t, invalidRec, http.StatusBadRequest)
}

func TestFactoryPackStoreAllowsDuplicateChecksumsAndLatestByID(t *testing.T) {
	store := newTestStore(t)
	data := readTestFile(t, buildTestFactoryPack(t))
	first, err := store.CreateFactoryPackVersion(FactoryPackVersion{PackName: "factory-core", PackVersion: "1", SchemaVersion: "1", CardSchemaVersion: pack.CardSchemaVersion, Checksum: "sha256:same", CompiledCaseCount: 3, Publisher: "alice", Data: data})
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	second, err := store.CreateFactoryPackVersion(FactoryPackVersion{PackName: "factory-core", PackVersion: "1", SchemaVersion: "1", CardSchemaVersion: pack.CardSchemaVersion, Checksum: "sha256:same", CompiledCaseCount: 3, Publisher: "bob", Data: data})
	if err != nil {
		t.Fatalf("create second duplicate checksum: %v", err)
	}
	if first.ID == second.ID {
		t.Fatalf("duplicate publishes reused id %d", first.ID)
	}
	latest, err := store.GetLatestFactoryPackVersion()
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if latest.ID != second.ID || latest.Publisher != "bob" {
		t.Fatalf("latest = %+v, want second publish", latest)
	}
}

func assertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d body = %s, want %d", rec.Code, rec.Body.String(), want)
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store, err := NewStore(db)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	return store
}

func buildTestFactoryPack(t *testing.T) string {
	t.Helper()
	sourceDir, err := filepath.Abs(filepath.FromSlash("../factory/compiler/testdata/cards"))
	if err != nil {
		t.Fatalf("resolve source cards: %v", err)
	}
	path := filepath.Join(t.TempDir(), pack.FileName)
	if _, err := compiler.CompilePack(sourceDir, path); err != nil {
		t.Fatalf("CompilePack() error = %v", err)
	}
	return path
}

func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func writeTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
