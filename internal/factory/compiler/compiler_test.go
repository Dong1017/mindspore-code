package compiler

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/mindspore-lab/mindspore-cli/internal/factory/pack"
)

func TestCompilePackBuildsSQLitePack(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), pack.FileName)
	summary, err := CompilePack("testdata/cards", outputPath)
	if err != nil {
		t.Fatalf("CompilePack() error = %v", err)
	}
	if summary.SourceCaseCount != 6 {
		t.Fatalf("SourceCaseCount = %d, want 6", summary.SourceCaseCount)
	}
	if summary.CompiledCaseCount != 3 {
		t.Fatalf("CompiledCaseCount = %d, want 3", summary.CompiledCaseCount)
	}
	if summary.DraftExcluded != 1 {
		t.Fatalf("DraftExcluded = %d, want 1", summary.DraftExcluded)
	}
	if summary.DeprecatedExcluded != 1 {
		t.Fatalf("DeprecatedExcluded = %d, want 1", summary.DeprecatedExcluded)
	}
	if summary.ArchivedExcluded != 1 {
		t.Fatalf("ArchivedExcluded = %d, want 1", summary.ArchivedExcluded)
	}
	if summary.Checksum == "" || !strings.HasPrefix(summary.Checksum, "sha256:") {
		t.Fatalf("Checksum = %q, want sha256 prefix", summary.Checksum)
	}

	db := openPack(t, outputPath)
	defer db.Close()
	if got := queryInt(t, db, `SELECT COUNT(*) FROM cases`); got != 3 {
		t.Fatalf("cases count = %d, want 3", got)
	}
	assertCaseExists(t, db, "stable-ascend-import")
	assertCaseExists(t, db, "stable-mindspore-compile")
	assertCaseMissing(t, db, "draft-excluded")
	assertCaseMissing(t, db, "deprecated-excluded")
	assertCaseMissing(t, db, "archived-excluded")

	manifest := readManifest(t, db)
	for _, key := range pack.RequiredManifestKeys {
		if manifest[key] == "" {
			t.Fatalf("manifest[%s] is empty", key)
		}
	}
	if manifest[pack.ManifestKeyCompiledCaseCount] != "3" {
		t.Fatalf("compiled manifest count = %q, want 3", manifest[pack.ManifestKeyCompiledCaseCount])
	}
	if manifest[pack.ManifestKeyChecksum] != summary.Checksum {
		t.Fatalf("manifest checksum = %q, summary checksum = %q", manifest[pack.ManifestKeyChecksum], summary.Checksum)
	}
	if got := queryInt(t, db, `SELECT COUNT(*) FROM keywords`); got == 0 {
		t.Fatalf("keywords count = 0, want > 0")
	}
	if got := queryInt(t, db, `SELECT COUNT(*) FROM patterns`); got == 0 {
		t.Fatalf("patterns count = 0, want > 0")
	}
	recomputed, err := pack.ComputeRuntimeChecksum(db)
	if err != nil {
		t.Fatalf("ComputeRuntimeChecksum() error = %v", err)
	}
	if recomputed != summary.Checksum {
		t.Fatalf("recomputed checksum = %q, summary checksum = %q", recomputed, summary.Checksum)
	}
}

func TestCompilePackInvalidCardFails(t *testing.T) {
	_, err := CompilePack("testdata/invalid", filepath.Join(t.TempDir(), pack.FileName))
	assertErrorContains(t, err, "invalid case.problem_type")
}

func TestCompilePackNoEligibleCardsFails(t *testing.T) {
	_, err := CompilePack("testdata/noeligible", filepath.Join(t.TempDir(), pack.FileName))
	assertErrorContains(t, err, "no eligible stable cards found")
}

func openPack(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite pack: %v", err)
	}
	return db
}

func queryInt(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var got int
	if err := db.QueryRow(query, args...).Scan(&got); err != nil {
		t.Fatalf("query %q: %v", query, err)
	}
	return got
}

func assertCaseExists(t *testing.T, db *sql.DB, id string) {
	t.Helper()
	if got := queryInt(t, db, `SELECT COUNT(*) FROM cases WHERE id = ?`, id); got != 1 {
		t.Fatalf("case %s count = %d, want 1", id, got)
	}
}

func assertCaseMissing(t *testing.T, db *sql.DB, id string) {
	t.Helper()
	if got := queryInt(t, db, `SELECT COUNT(*) FROM cases WHERE id = ?`, id); got != 0 {
		t.Fatalf("case %s count = %d, want 0", id, got)
	}
}

func readManifest(t *testing.T, db *sql.DB) map[string]string {
	t.Helper()
	rows, err := db.Query(`SELECT key, value FROM manifest`)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	defer rows.Close()
	manifest := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			t.Fatalf("scan manifest: %v", err)
		}
		manifest[key] = value
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("manifest rows: %v", err)
	}
	return manifest
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
