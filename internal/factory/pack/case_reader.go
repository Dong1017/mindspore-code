package pack

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

type matchCase struct {
	ID                  string
	Title               string
	ProblemType         string
	Stage               string
	RootCauseSummary    string
	FixSummary          string
	VerificationSummary string
	ConfidenceLevel     string
	LifecycleState      string
	Keywords            []string
	RegexPatterns       []string
	StackPatterns       []string
	NegativePatterns    []string
	Tags                []string
	Metadata            map[string][]string
	Advice              map[string][]string
}

func readMatchCases(path string) ([]matchCase, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open pack cases: %w", err)
	}
	defer db.Close()

	rows, err := db.Query(`SELECT id, title, problem_type, stage, root_cause_summary, fix_summary, verification_summary, confidence_level, lifecycle_state FROM cases ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("read cases: %w", err)
	}
	defer rows.Close()

	cases := make([]matchCase, 0)
	for rows.Next() {
		var c matchCase
		if err := rows.Scan(&c.ID, &c.Title, &c.ProblemType, &c.Stage, &c.RootCauseSummary, &c.FixSummary, &c.VerificationSummary, &c.ConfidenceLevel, &c.LifecycleState); err != nil {
			return nil, fmt.Errorf("scan case: %w", err)
		}
		c.Advice = make(map[string][]string)
		cases = append(cases, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read case rows: %w", err)
	}

	for i := range cases {
		if err := hydrateMatchCase(db, &cases[i]); err != nil {
			return nil, err
		}
	}
	return cases, nil
}

func hydrateMatchCase(db *sql.DB, c *matchCase) error {
	keywords, err := queryStrings(db, `SELECT keyword FROM keywords WHERE case_id = ? ORDER BY keyword`, c.ID)
	if err != nil {
		return fmt.Errorf("read keywords for %s: %w", c.ID, err)
	}
	c.Keywords = keywords

	patterns, err := queryPatterns(db, c.ID)
	if err != nil {
		return err
	}
	c.RegexPatterns = patterns["regex"]
	c.StackPatterns = patterns["stack_keyword"]
	c.NegativePatterns = patterns["negative"]

	tags, err := queryStrings(db, `SELECT tag FROM tags WHERE case_id = ? ORDER BY tag`, c.ID)
	if err != nil {
		return fmt.Errorf("read tags for %s: %w", c.ID, err)
	}
	c.Tags = tags

	metadata, err := queryMetadata(db, c.ID)
	if err != nil {
		return err
	}
	c.Metadata = metadata

	advice, err := queryAdvice(db, c.ID)
	if err != nil {
		return err
	}
	c.Advice = advice
	return nil
}

func queryStrings(db *sql.DB, query string, args ...any) ([]string, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		value = strings.TrimSpace(value)
		if value != "" {
			values = append(values, value)
		}
	}
	return values, rows.Err()
}

func queryPatterns(db *sql.DB, caseID string) (map[string][]string, error) {
	rows, err := db.Query(`SELECT pattern_type, pattern FROM patterns WHERE case_id = ? ORDER BY id`, caseID)
	if err != nil {
		return nil, fmt.Errorf("read patterns for %s: %w", caseID, err)
	}
	defer rows.Close()
	patterns := make(map[string][]string)
	for rows.Next() {
		var patternType, pattern string
		if err := rows.Scan(&patternType, &pattern); err != nil {
			return nil, fmt.Errorf("scan pattern for %s: %w", caseID, err)
		}
		patterns[patternType] = append(patterns[patternType], pattern)
	}
	return patterns, rows.Err()
}

func queryMetadata(db *sql.DB, caseID string) (map[string][]string, error) {
	rows, err := db.Query(`SELECT key, value FROM case_metadata WHERE case_id = ? ORDER BY key, value`, caseID)
	if err != nil {
		return nil, fmt.Errorf("read metadata for %s: %w", caseID, err)
	}
	defer rows.Close()
	metadata := make(map[string][]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("scan metadata for %s: %w", caseID, err)
		}
		value = strings.TrimSpace(value)
		if value != "" {
			metadata[key] = append(metadata[key], value)
		}
	}
	return metadata, rows.Err()
}

func queryAdvice(db *sql.DB, caseID string) (map[string][]string, error) {
	rows, err := db.Query(`SELECT advice_type, content FROM advice WHERE case_id = ? ORDER BY advice_type`, caseID)
	if err != nil {
		return nil, fmt.Errorf("read advice for %s: %w", caseID, err)
	}
	defer rows.Close()
	advice := make(map[string][]string)
	for rows.Next() {
		var adviceType, content string
		if err := rows.Scan(&adviceType, &content); err != nil {
			return nil, fmt.Errorf("scan advice for %s: %w", caseID, err)
		}
		for _, line := range strings.Split(content, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				advice[adviceType] = append(advice[adviceType], line)
			}
		}
	}
	return advice, rows.Err()
}
