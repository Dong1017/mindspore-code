package server

import (
	"database/sql"
	"errors"
	"time"
)

var ErrFactoryPackNotFound = errors.New("factory pack not found")

type FactoryPackVersion struct {
	ID                int
	PackName          string
	PackVersion       string
	SchemaVersion     string
	CardSchemaVersion string
	Checksum          string
	CompiledCaseCount int
	Publisher         string
	CreatedAt         time.Time
	Data              []byte
}

func (s *Store) CreateFactoryPackVersion(p FactoryPackVersion) (*FactoryPackVersion, error) {
	createdAt := p.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	res, err := s.db.Exec(`INSERT INTO factory_pack_versions (pack_name, pack_version, schema_version, card_schema_version, checksum, compiled_case_count, publisher, pack_blob, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.PackName,
		p.PackVersion,
		p.SchemaVersion,
		p.CardSchemaVersion,
		p.Checksum,
		p.CompiledCaseCount,
		p.Publisher,
		p.Data,
		createdAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.GetFactoryPackVersion(int(id))
}

func (s *Store) GetLatestFactoryPackVersion() (*FactoryPackVersion, error) {
	return s.scanFactoryPackVersion(s.db.QueryRow(`SELECT id, pack_name, pack_version, schema_version, card_schema_version, checksum, compiled_case_count, publisher, pack_blob, created_at FROM factory_pack_versions ORDER BY created_at DESC, id DESC LIMIT 1`))
}

func (s *Store) GetFactoryPackVersion(id int) (*FactoryPackVersion, error) {
	return s.scanFactoryPackVersion(s.db.QueryRow(`SELECT id, pack_name, pack_version, schema_version, card_schema_version, checksum, compiled_case_count, publisher, pack_blob, created_at FROM factory_pack_versions WHERE id = ?`, id))
}

func (s *Store) scanFactoryPackVersion(row *sql.Row) (*FactoryPackVersion, error) {
	var p FactoryPackVersion
	var createdAt string
	if err := row.Scan(&p.ID, &p.PackName, &p.PackVersion, &p.SchemaVersion, &p.CardSchemaVersion, &p.Checksum, &p.CompiledCaseCount, &p.Publisher, &p.Data, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrFactoryPackNotFound
		}
		return nil, err
	}
	parsed, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, err
	}
	p.CreatedAt = parsed
	return &p, nil
}
