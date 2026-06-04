package pack

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
)

type checksumTable struct {
	name    string
	columns []string
	orderBy []string
}

var checksumTables = []checksumTable{
	{name: "manifest", columns: []string{"key", "value"}, orderBy: []string{"key"}},
	{name: "cases", columns: []string{"id", "kind", "title", "problem_type", "stage", "root_cause_summary", "fix_summary", "verification_summary", "confidence_level", "lifecycle_state", "source_card_hash"}, orderBy: []string{"id"}},
	{name: "patterns", columns: []string{"id", "case_id", "pattern_type", "pattern", "weight"}, orderBy: []string{"case_id", "pattern_type", "pattern", "id"}},
	{name: "keywords", columns: []string{"case_id", "keyword", "weight"}, orderBy: []string{"case_id", "keyword"}},
	{name: "tags", columns: []string{"case_id", "tag"}, orderBy: []string{"case_id", "tag"}},
	{name: "advice", columns: []string{"case_id", "advice_type", "content"}, orderBy: []string{"case_id", "advice_type"}},
	{name: "provenance", columns: []string{"case_id", "key", "value"}, orderBy: []string{"case_id", "key", "value"}},
	{name: "case_metadata", columns: []string{"case_id", "key", "value"}, orderBy: []string{"case_id", "key", "value"}},
	{name: "case_applicability", columns: []string{"case_id", "key", "value"}, orderBy: []string{"case_id", "key", "value"}},
}

func ComputeRuntimeChecksum(db *sql.DB) (string, error) {
	h := sha256.New()
	for _, table := range checksumTables {
		exists, err := tableExists(db, table.name)
		if err != nil {
			return "", err
		}
		if !exists {
			continue
		}
		h.Write([]byte("table:" + table.name + "\n"))
		query := checksumQuery(table)
		rows, err := db.Query(query)
		if err != nil {
			return "", fmt.Errorf("read checksum table %s: %w", table.name, err)
		}
		values := make([]sql.NullString, len(table.columns))
		scan := make([]any, len(values))
		for i := range values {
			scan[i] = &values[i]
		}
		for rows.Next() {
			if err := rows.Scan(scan...); err != nil {
				rows.Close()
				return "", fmt.Errorf("scan checksum table %s: %w", table.name, err)
			}
			if table.name == "manifest" && values[0].String == ManifestKeyChecksum {
				continue
			}
			for i, column := range table.columns {
				h.Write([]byte(column))
				h.Write([]byte("="))
				if values[i].Valid {
					h.Write([]byte(values[i].String))
				}
				h.Write([]byte{0})
			}
			h.Write([]byte("\n"))
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return "", fmt.Errorf("read checksum rows %s: %w", table.name, err)
		}
		rows.Close()
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

func tableExists(db *sql.DB, table string) (bool, error) {
	var name string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect table %s: %w", table, err)
	}
	return true, nil
}

func checksumQuery(table checksumTable) string {
	columns := make([]string, 0, len(table.columns))
	for _, column := range table.columns {
		columns = append(columns, quoteIdent(column))
	}
	orderBy := make([]string, 0, len(table.orderBy))
	for _, column := range table.orderBy {
		orderBy = append(orderBy, quoteIdent(column))
	}
	return fmt.Sprintf("SELECT %s FROM %s ORDER BY %s", strings.Join(columns, ", "), quoteIdent(table.name), strings.Join(orderBy, ", "))
}

func quoteIdent(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
