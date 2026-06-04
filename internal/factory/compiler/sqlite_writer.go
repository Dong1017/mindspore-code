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
	if _, err := db.Exec(`INSERT INTO cases(id, kind, title, problem_type, stage, root_cause_summary, fix_summary, verification_summary, confidence_level, lifecycle_state, source_card_hash) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID,
		c.Kind,
		c.Title,
		c.Case.ProblemType,
		c.Case.Stage,
		c.Guidance.Diagnosis,
		c.Guidance.Fix,
		c.Guidance.Verification,
		c.Governance.Confidence,
		c.Governance.Lifecycle,
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
		"case.domain":            {c.Case.Domain},
		"hardware.accelerator":   {c.Case.Hardware},
		"runtime.cann_version":   {c.Case.Environment.Runtime.CANNVersion},
		"runtime.python_version": {c.Case.Environment.Runtime.PythonVersion},
		"model.pattern":          {c.Case.Environment.Model.Pattern},
		"model.execution_mode":   {c.Case.Environment.Model.ExecutionMode},
		"model.optimization":     {c.Case.Environment.Model.Optimization},
		"model.input_reuse":      {c.Case.Environment.Model.InputReuse},
		"model.dtype":            {c.Case.Environment.Model.DType},
	}
	for _, framework := range c.Case.Environment.Frameworks {
		entries["framework.name"] = append(entries["framework.name"], framework.Name)
		entries["framework.version"] = append(entries["framework.version"], framework.Version)
		entries["framework.branch"] = append(entries["framework.branch"], framework.Branch)
		entries["framework.commit"] = append(entries["framework.commit"], framework.Commit)
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
	for idx, pattern := range nonEmptySorted(c.Match.Regex) {
		if _, err := db.Exec(`INSERT INTO patterns(id, case_id, pattern_type, pattern, weight) VALUES(?, ?, ?, ?, ?)`, fmt.Sprintf("%s:regex:%d", c.ID, idx+1), c.ID, "regex", pattern, 5.0); err != nil {
			return fmt.Errorf("write regex pattern for %s: %w", c.ID, err)
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
		"symptom":               c.Guidance.Symptom,
		"trigger_signals":       strings.Join(c.Guidance.TriggerSignals, "\n"),
		"representative_errors": strings.Join(c.Guidance.RepresentativeErrors, "\n"),
		"explanation":           c.Guidance.Diagnosis,
		"diagnosis_details":     strings.Join(c.Guidance.DiagnosisDetails, "\n"),
		"fix_summary":           c.Guidance.Fix,
		"fix_steps":             strings.Join(c.Guidance.Actions, "\n"),
		"why_it_works":          strings.Join(c.Guidance.WhyItWorks, "\n"),
		"verification":          c.Guidance.Verification,
		"non_causes":            strings.Join(c.Guidance.NonCauses, "\n"),
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
	for _, expected := range nonEmptySorted(c.Provenance.ExpectedBehavior) {
		if _, err := db.Exec(`INSERT INTO provenance(case_id, key, value) VALUES(?, ?, ?)`, c.ID, "expected_behavior", expected); err != nil {
			return fmt.Errorf("write expected behavior for %s: %w", c.ID, err)
		}
	}
	for _, test := range nonEmptySorted(c.Provenance.RegressionTests) {
		if _, err := db.Exec(`INSERT INTO provenance(case_id, key, value) VALUES(?, ?, ?)`, c.ID, "regression_test", test); err != nil {
			return fmt.Errorf("write regression test for %s: %w", c.ID, err)
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
		"affected": c.Case.Environment.Affected,
		"fixed_by": c.Case.Environment.FixedBy,
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
