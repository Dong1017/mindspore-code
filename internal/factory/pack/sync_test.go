package pack_test

import (
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mindspore-lab/mindspore-cli/internal/factory/compiler"
	"github.com/mindspore-lab/mindspore-cli/internal/factory/pack"
	_ "modernc.org/sqlite"
)

func TestSyncFromValidLocalPackInstallsAndLoads(t *testing.T) {
	source := buildSyncPack(t)
	dest := filepath.Join(t.TempDir(), pack.FileName)
	result, err := pack.Sync(pack.SyncConfig{SourcePath: source, DestPath: dest})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if result.SourcePath != source {
		t.Fatalf("SourcePath = %q, want %q", result.SourcePath, source)
	}
	if result.DestPath != dest {
		t.Fatalf("DestPath = %q, want %q", result.DestPath, dest)
	}
	if result.PackName != pack.Name || result.CardSchemaVersion != pack.CardSchemaVersion || result.CompiledCaseCount != 3 {
		t.Fatalf("result = %#v, want valid summary", result)
	}
	loaded, err := pack.Load(dest)
	if err != nil {
		t.Fatalf("Load(installed) error = %v", err)
	}
	if loaded.Manifest.Checksum != result.Checksum {
		t.Fatalf("installed checksum = %q, result checksum = %q", loaded.Manifest.Checksum, result.Checksum)
	}
}

func TestSyncReplacesExistingDestinationPack(t *testing.T) {
	source := buildSyncPack(t)
	dest := filepath.Join(t.TempDir(), pack.FileName)
	if err := os.WriteFile(dest, []byte("old invalid pack"), 0o600); err != nil {
		t.Fatalf("write existing dest: %v", err)
	}
	if _, err := pack.Sync(pack.SyncConfig{SourcePath: source, DestPath: dest}); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if _, err := pack.Load(dest); err != nil {
		t.Fatalf("Load(replaced) error = %v", err)
	}
}

func TestSyncDoesNotClobberExistingRealBakFile(t *testing.T) {
	source := buildSyncPack(t)
	dir := t.TempDir()
	dest := filepath.Join(dir, pack.FileName)
	if err := os.WriteFile(dest, []byte("old invalid pack"), 0o600); err != nil {
		t.Fatalf("write existing dest: %v", err)
	}
	realBak := dest + ".bak"
	if err := os.WriteFile(realBak, []byte("user backup"), 0o600); err != nil {
		t.Fatalf("write real bak: %v", err)
	}
	if _, err := pack.Sync(pack.SyncConfig{SourcePath: source, DestPath: dest}); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if _, err := pack.Load(dest); err != nil {
		t.Fatalf("Load(replaced) error = %v", err)
	}
	assertFileContent(t, realBak, "user backup")
}

func TestSyncFileURLSourcePathWorks(t *testing.T) {
	source := buildSyncPack(t)
	dest := filepath.Join(t.TempDir(), pack.FileName)
	sourceURL := url.URL{Scheme: "file", Path: filepath.ToSlash(source)}
	result, err := pack.Sync(pack.SyncConfig{SourcePath: sourceURL.String(), DestPath: dest})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if result.SourcePath != source {
		t.Fatalf("SourcePath = %q, want %q", result.SourcePath, source)
	}
	if _, err := pack.Load(dest); err != nil {
		t.Fatalf("Load(installed) error = %v", err)
	}
}

func TestSyncInvalidSourceDoesNotReplaceExistingDestination(t *testing.T) {
	dest := filepath.Join(t.TempDir(), pack.FileName)
	writeMarkerPack(t, dest, "existing")
	source := filepath.Join(t.TempDir(), "invalid.pack")
	writeMarkerPack(t, source, "invalid")
	_, err := pack.Sync(pack.SyncConfig{SourcePath: source, DestPath: dest})
	assertPackErrorContains(t, err, "validate source pack")
	assertFileContent(t, dest, "existing")
}

func TestSyncIncompatiblePackSchemaPreservesExistingDestination(t *testing.T) {
	dest := filepath.Join(t.TempDir(), pack.FileName)
	writeMarkerPack(t, dest, "existing")
	source := buildSyncPack(t)
	updateSyncManifestKey(t, source, pack.ManifestKeySchemaVersion, "999")
	_, err := pack.Sync(pack.SyncConfig{SourcePath: source, DestPath: dest})
	assertPackErrorContains(t, err, "unsupported pack schema_version")
	assertFileContent(t, dest, "existing")
}

func TestSyncIncompatibleCardSchemaPreservesExistingDestination(t *testing.T) {
	dest := filepath.Join(t.TempDir(), pack.FileName)
	writeMarkerPack(t, dest, "existing")
	source := buildSyncPack(t)
	updateSyncManifestKey(t, source, pack.ManifestKeyCardSchemaVersion, "known_issue/v0.4")
	_, err := pack.Sync(pack.SyncConfig{SourcePath: source, DestPath: dest})
	assertPackErrorContains(t, err, "unsupported card_schema_version")
	assertFileContent(t, dest, "existing")
}

func TestSyncChecksumMismatchPreservesExistingDestination(t *testing.T) {
	dest := filepath.Join(t.TempDir(), pack.FileName)
	writeMarkerPack(t, dest, "existing")
	source := buildSyncPack(t)
	db := openSyncDB(t, source)
	if _, err := db.Exec(`UPDATE cases SET title = ? WHERE id = ?`, "tampered", "stable-ascend-import"); err != nil {
		db.Close()
		t.Fatalf("tamper source: %v", err)
	}
	db.Close()
	_, err := pack.Sync(pack.SyncConfig{SourcePath: source, DestPath: dest})
	assertPackErrorContains(t, err, "pack checksum mismatch")
	assertFileContent(t, dest, "existing")
}

func TestSyncMissingSourceReturnsClearError(t *testing.T) {
	_, err := pack.Sync(pack.SyncConfig{SourcePath: filepath.Join(t.TempDir(), "missing.pack"), DestPath: filepath.Join(t.TempDir(), pack.FileName)})
	assertPackErrorContains(t, err, "source pack does not exist")
}

func TestSyncEmptySourceReturnsClearError(t *testing.T) {
	_, err := pack.Sync(pack.SyncConfig{DestPath: filepath.Join(t.TempDir(), pack.FileName)})
	assertPackErrorContains(t, err, "factory pack source is required")
}

func buildSyncPack(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), pack.FileName)
	if _, err := compiler.CompilePack(filepath.FromSlash("../compiler/testdata/cards"), path); err != nil {
		t.Fatalf("CompilePack() error = %v", err)
	}
	return path
}

func openSyncDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite pack: %v", err)
	}
	return db
}

func updateSyncManifestKey(t *testing.T, path, key, value string) {
	t.Helper()
	db := openSyncDB(t, path)
	defer db.Close()
	if _, err := db.Exec(`UPDATE manifest SET value = ? WHERE key = ?`, value, key); err != nil {
		t.Fatalf("update manifest key: %v", err)
	}
}

func writeMarkerPack(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create marker dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatalf("write marker pack: %v", err)
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(data) != want {
		t.Fatalf("content = %q, want %q", string(data), want)
	}
}

func assertPackErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %q, want substring %q", err.Error(), want)
	}
}
