package compiler

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/mindspore-lab/mindspore-cli/internal/factory/card"
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
	if manifest[pack.ManifestKeyCardSchemaVersion] != pack.CardSchemaVersion {
		t.Fatalf("card schema version = %q, want %q", manifest[pack.ManifestKeyCardSchemaVersion], pack.CardSchemaVersion)
	}
	if manifest[pack.ManifestKeyChecksum] != summary.Checksum {
		t.Fatalf("manifest checksum = %q, summary checksum = %q", manifest[pack.ManifestKeyChecksum], summary.Checksum)
	}
	if got := queryInt(t, db, `SELECT COUNT(*) FROM patterns WHERE pattern_type = 'negative'`); got != 0 {
		t.Fatalf("negative pattern count = %d, want 0", got)
	}
	if got := queryInt(t, db, `SELECT COUNT(*) FROM advice WHERE advice_type = 'non_causes'`); got == 0 {
		t.Fatalf("non_causes advice count = 0, want > 0")
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
	dir := t.TempDir()
	invalid := compilerTestCard("invalid-card")
	invalid.Case.ProblemType = "bad"
	writeCompilerTestCard(t, dir, invalid)
	_, err := CompilePack(dir, filepath.Join(t.TempDir(), pack.FileName))
	assertErrorContains(t, err, "invalid case.problem_type")
}

func TestCompilePackNoEligibleCardsFails(t *testing.T) {
	dir := t.TempDir()
	draft := compilerTestCard("only-draft")
	draft.Governance = card.Governance{Confidence: card.ConfidenceBootstrap, Lifecycle: card.LifecycleDraft, ReviewStatus: card.ReviewPending}
	writeCompilerTestCard(t, dir, draft)
	_, err := CompilePack(dir, filepath.Join(t.TempDir(), pack.FileName))
	assertErrorContains(t, err, "no eligible stable cards found")
}

func compilerTestCard(id string) *card.KnownIssueCard {
	return &card.KnownIssueCard{
		SchemaVersion: card.SchemaVersionKnownIssueV05,
		Kind:          card.KindKnownIssue,
		ID:            id,
		Title:         "torch_npu import fails when CANN is missing",
		Tags:          []string{"torch_npu"},
		Case: card.Case{
			ProblemType: card.ProblemTypeFailure,
			Stage:       card.StageImport,
			Domain:      card.DomainTorchNPU,
			Hardware:    card.HardwareAscend,
		},
		Match: card.Match{Keywords: []string{"torch_npu"}},
		Guidance: card.Guidance{
			Symptom:      "torch_npu import fails",
			Diagnosis:    "CANN runtime is not visible",
			Verification: "Run import smoke test",
		},
		Provenance: card.Provenance{
			References:       []string{"issue-123"},
			ExpectedBehavior: []string{"torch_npu imports"},
		},
		Governance: card.Governance{
			Confidence:   card.ConfidenceObserved,
			Lifecycle:    card.LifecycleStable,
			ReviewStatus: card.ReviewApproved,
			Rationale:    "manual review passed",
		},
	}
}

func writeCompilerTestCard(t *testing.T, dir string, known *card.KnownIssueCard) {
	t.Helper()
	if _, err := card.WriteDraftYAML(known, dir); err != nil {
		t.Fatalf("WriteDraftYAML() error = %v", err)
	}
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
