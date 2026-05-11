package pack_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mindspore-lab/mindspore-cli/internal/factory/compiler"
	"github.com/mindspore-lab/mindspore-cli/internal/factory/pack"
	_ "modernc.org/sqlite"
)

func TestLoadAndInspectGeneratedPack(t *testing.T) {
	path := buildTestPack(t)
	loaded, err := pack.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Path != path {
		t.Fatalf("Pack.Path = %q, want %q", loaded.Path, path)
	}
	if loaded.Manifest.PackName != pack.Name {
		t.Fatalf("PackName = %q, want %q", loaded.Manifest.PackName, pack.Name)
	}
	if loaded.Manifest.SchemaVersion != pack.SchemaVersion {
		t.Fatalf("SchemaVersion = %q, want %q", loaded.Manifest.SchemaVersion, pack.SchemaVersion)
	}
	if loaded.Manifest.CardSchemaVersion != pack.CardSchemaVersion {
		t.Fatalf("CardSchemaVersion = %q, want %q", loaded.Manifest.CardSchemaVersion, pack.CardSchemaVersion)
	}
	if loaded.Manifest.SourceCaseCount != 6 {
		t.Fatalf("SourceCaseCount = %d, want 6", loaded.Manifest.SourceCaseCount)
	}
	if loaded.Manifest.CompiledCaseCount != 3 {
		t.Fatalf("CompiledCaseCount = %d, want 3", loaded.Manifest.CompiledCaseCount)
	}

	manifest, err := pack.Inspect(path)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if manifest.Checksum == "" {
		t.Fatalf("Checksum is empty")
	}
}

func TestLoadDefaultUsesOverridePath(t *testing.T) {
	path := buildTestPack(t)
	loaded, err := pack.LoadDefault(pack.LoadConfig{PackPath: path})
	if err != nil {
		t.Fatalf("LoadDefault() error = %v", err)
	}
	if loaded.Path != path {
		t.Fatalf("LoadDefault path = %q, want %q", loaded.Path, path)
	}
}

func TestDefaultPackPath(t *testing.T) {
	path, err := pack.DefaultPackPath()
	if err != nil {
		t.Fatalf("DefaultPackPath() error = %v", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error = %v", err)
	}
	want := filepath.Join(home, ".mscli", "factory", pack.FileName)
	if path != want {
		t.Fatalf("DefaultPackPath() = %q, want %q", path, want)
	}
}

func TestLoadMissingPackFails(t *testing.T) {
	_, err := pack.Load(filepath.Join(t.TempDir(), pack.FileName))
	assertErrorContains(t, err, "read manifest")
}

func TestLoadTamperedPackFailsChecksumValidation(t *testing.T) {
	path := buildTestPack(t)
	db := openTestPack(t, path)
	if _, err := db.Exec(`UPDATE cases SET title = ? WHERE id = ?`, "tampered title", "stable-ascend-import"); err != nil {
		db.Close()
		t.Fatalf("tamper pack: %v", err)
	}
	db.Close()

	_, err := pack.Load(path)
	assertErrorContains(t, err, "pack checksum mismatch")
}

func TestInspectMissingManifestFieldFails(t *testing.T) {
	path := buildTestPack(t)
	deleteManifestKey(t, path, pack.ManifestKeyChecksum)
	_, err := pack.Inspect(path)
	assertErrorContains(t, err, "manifest missing required field: checksum")
}

func TestInspectIncompatibleSchemaFails(t *testing.T) {
	path := buildTestPack(t)
	updateManifestKey(t, path, pack.ManifestKeySchemaVersion, "999")
	_, err := pack.Inspect(path)
	assertErrorContains(t, err, "unsupported pack schema_version")
}

func TestInspectMissingCardSchemaVersionFails(t *testing.T) {
	path := buildTestPack(t)
	deleteManifestKey(t, path, pack.ManifestKeyCardSchemaVersion)
	_, err := pack.Inspect(path)
	assertErrorContains(t, err, "manifest missing required field: card_schema_version")
}

func TestInspectIncompatibleCardSchemaVersionFails(t *testing.T) {
	path := buildTestPack(t)
	updateManifestKey(t, path, pack.ManifestKeyCardSchemaVersion, "known_issue/v0.4")
	_, err := pack.Inspect(path)
	assertErrorContains(t, err, "unsupported card_schema_version")
}

func TestInspectInvalidChecksumFails(t *testing.T) {
	path := buildTestPack(t)
	updateManifestKey(t, path, pack.ManifestKeyChecksum, "not-a-checksum")
	_, err := pack.Inspect(path)
	assertErrorContains(t, err, "invalid manifest checksum format")
}

func TestInspectCorruptPackFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), pack.FileName)
	if err := os.WriteFile(path, []byte("not sqlite"), 0o600); err != nil {
		t.Fatalf("write corrupt pack: %v", err)
	}
	_, err := pack.Inspect(path)
	assertErrorContains(t, err, "read manifest")
}

func buildTestPack(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), pack.FileName)
	if _, err := compiler.CompilePack(filepath.FromSlash("../compiler/testdata/cards"), path); err != nil {
		t.Fatalf("CompilePack() error = %v", err)
	}
	return path
}

func openTestPack(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite pack: %v", err)
	}
	return db
}

func deleteManifestKey(t *testing.T, path, key string) {
	t.Helper()
	db := openTestPack(t, path)
	defer db.Close()
	if _, err := db.Exec(`DELETE FROM manifest WHERE key = ?`, key); err != nil {
		t.Fatalf("delete manifest key: %v", err)
	}
}

func updateManifestKey(t *testing.T, path, key, value string) {
	t.Helper()
	db := openTestPack(t, path)
	defer db.Close()
	if _, err := db.Exec(`UPDATE manifest SET value = ? WHERE key = ?`, value, key); err != nil {
		t.Fatalf("update manifest key: %v", err)
	}
}

func assertErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %q, want substring %q", err.Error(), want)
	}
}
