package pack

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

func Inspect(path string) (*Manifest, error) {
	if err := requirePath(path); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open pack: %w", err)
	}
	defer db.Close()

	values, err := readManifest(db)
	if err != nil {
		return nil, err
	}
	manifest, err := parseManifest(values)
	if err != nil {
		return nil, err
	}
	return manifest, nil
}

func readManifest(db *sql.DB) (map[string]string, error) {
	rows, err := db.Query(`SELECT key, value FROM manifest`)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	defer rows.Close()

	values := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("scan manifest: %w", err)
		}
		values[key] = value
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read manifest rows: %w", err)
	}
	return values, nil
}
