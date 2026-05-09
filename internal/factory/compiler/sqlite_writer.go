package compiler

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	// Factory pack build/load uses modernc.org/sqlite so it works when CGO is disabled; the existing go-sqlite3 dependency requires CGO.
	_ "modernc.org/sqlite"

	"github.com/mindspore-lab/mindspore-cli/internal/factory/card"
	"github.com/mindspore-lab/mindspore-cli/internal/factory/pack"
)

func writeSQLitePack(outputPath string, cards []*card.KnownIssueCard, manifest map[string]string) error {
	if err := requireManifestFields(manifest); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o700); err != nil {
		return fmt.Errorf("create pack output dir: %w", err)
	}
	if err := os.Remove(outputPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("replace existing pack: %w", err)
	}

	db, err := sql.Open("sqlite", outputPath)
	if err != nil {
		return fmt.Errorf("open sqlite pack: %w", err)
	}
	defer db.Close()

	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		return fmt.Errorf("enable sqlite foreign keys: %w", err)
	}
	if err := createTables(db); err != nil {
		return err
	}
	if err := writeManifest(db, manifest); err != nil {
		return err
	}
	for _, c := range cards {
		if err := writeCase(db, c); err != nil {
			return err
		}
	}
	return nil
}

func createTables(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE manifest(key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
		`CREATE TABLE cases(
			id TEXT PRIMARY KEY,
			kind TEXT NOT NULL,
			title TEXT NOT NULL,
			problem_type TEXT NOT NULL,
			stage TEXT NOT NULL,
			root_cause_summary TEXT,
			fix_summary TEXT,
			verification_summary TEXT,
			confidence_level TEXT NOT NULL,
			lifecycle_state TEXT NOT NULL,
			source_card_hash TEXT
		)`,
		`CREATE TABLE case_metadata(
			case_id TEXT NOT NULL,
			key TEXT NOT NULL,
			value TEXT NOT NULL,
			FOREIGN KEY(case_id) REFERENCES cases(id)
		)`,
		`CREATE TABLE patterns(
			id TEXT PRIMARY KEY,
			case_id TEXT NOT NULL,
			pattern_type TEXT NOT NULL,
			pattern TEXT NOT NULL,
			weight REAL NOT NULL,
			FOREIGN KEY(case_id) REFERENCES cases(id)
		)`,
		`CREATE TABLE keywords(
			case_id TEXT NOT NULL,
			keyword TEXT NOT NULL,
			weight REAL NOT NULL,
			FOREIGN KEY(case_id) REFERENCES cases(id)
		)`,
		`CREATE TABLE tags(
			case_id TEXT NOT NULL,
			tag TEXT NOT NULL,
			FOREIGN KEY(case_id) REFERENCES cases(id)
		)`,
		`CREATE TABLE advice(
			case_id TEXT NOT NULL,
			advice_type TEXT NOT NULL,
			content TEXT NOT NULL,
			FOREIGN KEY(case_id) REFERENCES cases(id)
		)`,
		`CREATE TABLE provenance(
			case_id TEXT NOT NULL,
			key TEXT NOT NULL,
			value TEXT NOT NULL,
			FOREIGN KEY(case_id) REFERENCES cases(id)
		)`,
		`CREATE TABLE case_applicability(
			case_id TEXT NOT NULL,
			key TEXT NOT NULL,
			value TEXT NOT NULL,
			FOREIGN KEY(case_id) REFERENCES cases(id)
		)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return fmt.Errorf("create sqlite table: %w", err)
		}
	}
	return nil
}

func writeManifest(db *sql.DB, manifest map[string]string) error {
	keys := make([]string, 0, len(manifest))
	for key := range manifest {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, err := db.Exec(`INSERT INTO manifest(key, value) VALUES(?, ?)`, key, manifest[key]); err != nil {
			return fmt.Errorf("write manifest %s: %w", key, err)
		}
	}
	return nil
}

func writeCase(db *sql.DB, c *card.KnownIssueCard) error {
	verificationSummary := strings.Join(c.Verification.Checks, "\n")
	if _, err := db.Exec(`INSERT INTO cases(id, kind, title, problem_type, stage, root_cause_summary, fix_summary, verification_summary, confidence_level, lifecycle_state, source_card_hash) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID,
		c.Kind,
		c.Title,
		c.Problem.ProblemType,
		c.Problem.Stage,
		c.Diagnosis.RootCause,
		c.Fix.Summary,
		verificationSummary,
		c.Confidence.Level,
		c.Lifecycle.State,
		caseSourceHash(c),
	); err != nil {
		return fmt.Errorf("write case %s: %w", c.ID, err)
	}
	if err := writeMetadata(db, c); err != nil {
		return err
	}
	if err := writeKeywords(db, c); err != nil {
		return err
	}
	if err := writePatterns(db, c); err != nil {
		return err
	}
	if err := writeTags(db, c); err != nil {
		return err
	}
	if err := writeAdvice(db, c); err != nil {
		return err
	}
	if err := writeProvenance(db, c); err != nil {
		return err
	}
	if err := writeApplicability(db, c); err != nil {
		return err
	}
	return nil
}

func writeMetadata(db *sql.DB, c *card.KnownIssueCard) error {
	entries := map[string][]string{
		"hardware.accelerator": {c.Environment.Hardware.Accelerator},
		"runtime.cann_version": {c.Environment.Runtime.CANNVersion},
	}
	for _, framework := range c.Environment.Frameworks {
		entries["framework.name"] = append(entries["framework.name"], framework.Name)
	}
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		for _, value := range nonEmptySorted(entries[key]) {
			if _, err := db.Exec(`INSERT INTO case_metadata(case_id, key, value) VALUES(?, ?, ?)`, c.ID, key, value); err != nil {
				return fmt.Errorf("write metadata for %s: %w", c.ID, err)
			}
		}
	}
	return nil
}

func writeKeywords(db *sql.DB, c *card.KnownIssueCard) error {
	for _, keyword := range nonEmptySorted(c.Match.Keywords) {
		if _, err := db.Exec(`INSERT INTO keywords(case_id, keyword, weight) VALUES(?, ?, ?)`, c.ID, keyword, 2.0); err != nil {
			return fmt.Errorf("write keyword for %s: %w", c.ID, err)
		}
	}
	return nil
}

func writePatterns(db *sql.DB, c *card.KnownIssueCard) error {
	idx := 0
	for _, pattern := range nonEmptySorted(c.Match.Regex) {
		idx++
		if _, err := db.Exec(`INSERT INTO patterns(id, case_id, pattern_type, pattern, weight) VALUES(?, ?, ?, ?, ?)`, fmt.Sprintf("%s:regex:%d", c.ID, idx), c.ID, "regex", pattern, 5.0); err != nil {
			return fmt.Errorf("write regex pattern for %s: %w", c.ID, err)
		}
	}
	for _, pattern := range nonEmptySorted(c.Match.StackKeywords) {
		idx++
		if _, err := db.Exec(`INSERT INTO patterns(id, case_id, pattern_type, pattern, weight) VALUES(?, ?, ?, ?, ?)`, fmt.Sprintf("%s:stack:%d", c.ID, idx), c.ID, "stack_keyword", pattern, 4.0); err != nil {
			return fmt.Errorf("write stack pattern for %s: %w", c.ID, err)
		}
	}
	for _, pattern := range nonEmptySorted(c.Match.NegativePatterns) {
		idx++
		if _, err := db.Exec(`INSERT INTO patterns(id, case_id, pattern_type, pattern, weight) VALUES(?, ?, ?, ?, ?)`, fmt.Sprintf("%s:negative:%d", c.ID, idx), c.ID, "negative", pattern, -4.0); err != nil {
			return fmt.Errorf("write negative pattern for %s: %w", c.ID, err)
		}
	}
	return nil
}

func writeTags(db *sql.DB, c *card.KnownIssueCard) error {
	for _, tag := range nonEmptySorted(c.Tags) {
		if _, err := db.Exec(`INSERT INTO tags(case_id, tag) VALUES(?, ?)`, c.ID, tag); err != nil {
			return fmt.Errorf("write tag for %s: %w", c.ID, err)
		}
	}
	return nil
}

func writeAdvice(db *sql.DB, c *card.KnownIssueCard) error {
	advice := map[string]string{
		"explanation":      c.Diagnosis.Explanation,
		"scope_note":       c.Diagnosis.ScopeNote,
		"next_checks":      strings.Join(c.Diagnosis.SuggestedNextChecks, "\n"),
		"fix_summary":      c.Fix.Summary,
		"fix_steps":        strings.Join(c.Fix.Steps, "\n"),
		"fix_template":     c.Fix.Template,
		"verification":     strings.Join(c.Verification.Checks, "\n"),
		"expected_result":  c.Verification.ExpectedResult,
		"missing_evidence": strings.Join(c.Diagnosis.MissingEvidence, "\n"),
		"conflicts":        strings.Join(c.Diagnosis.ConflictingSignals, "\n"),
	}
	keys := make([]string, 0, len(advice))
	for key, value := range advice {
		if strings.TrimSpace(value) != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, err := db.Exec(`INSERT INTO advice(case_id, advice_type, content) VALUES(?, ?, ?)`, c.ID, key, advice[key]); err != nil {
			return fmt.Errorf("write advice for %s: %w", c.ID, err)
		}
	}
	return nil
}

func writeProvenance(db *sql.DB, c *card.KnownIssueCard) error {
	for _, ref := range nonEmptySorted(c.Provenance.References) {
		if _, err := db.Exec(`INSERT INTO provenance(case_id, key, value) VALUES(?, ?, ?)`, c.ID, "reference", ref); err != nil {
			return fmt.Errorf("write provenance for %s: %w", c.ID, err)
		}
	}
	if strings.TrimSpace(c.Provenance.Notes) != "" {
		if _, err := db.Exec(`INSERT INTO provenance(case_id, key, value) VALUES(?, ?, ?)`, c.ID, "notes", c.Provenance.Notes); err != nil {
			return fmt.Errorf("write provenance notes for %s: %w", c.ID, err)
		}
	}
	return nil
}

func writeApplicability(db *sql.DB, c *card.KnownIssueCard) error {
	entries := map[string][]string{
		"affected_versions": c.Applicability.AffectedVersions,
		"affected_branches": c.Applicability.AffectedBranches,
		"affected_commits":  c.Applicability.AffectedCommits,
		"introduced_by":     c.Applicability.IntroducedBy,
		"fixed_by":          c.Applicability.FixedBy,
	}
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		for _, value := range nonEmptySorted(entries[key]) {
			if _, err := db.Exec(`INSERT INTO case_applicability(case_id, key, value) VALUES(?, ?, ?)`, c.ID, key, value); err != nil {
				return fmt.Errorf("write applicability for %s: %w", c.ID, err)
			}
		}
	}
	return nil
}

func updateSQLitePackChecksum(outputPath string) (string, error) {
	db, err := sql.Open("sqlite", outputPath)
	if err != nil {
		return "", fmt.Errorf("open sqlite pack for checksum: %w", err)
	}
	defer db.Close()
	checksum, err := pack.ComputeRuntimeChecksum(db)
	if err != nil {
		return "", err
	}
	if _, err := db.Exec(`UPDATE manifest SET value = ? WHERE key = ?`, checksum, pack.ManifestKeyChecksum); err != nil {
		return "", fmt.Errorf("write pack checksum: %w", err)
	}
	return checksum, nil
}

func nonEmptySorted(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func caseSourceHash(c *card.KnownIssueCard) string {
	return c.ID
}
