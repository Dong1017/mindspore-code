package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/mindspore-lab/mindspore-cli/internal/factory/pack"
)

const maxFactoryPackUploadBytes = 50 << 20

type factoryPackMetadataResponse struct {
	ID                int    `json:"id"`
	PackName          string `json:"pack_name"`
	PackVersion       string `json:"pack_version"`
	SchemaVersion     string `json:"schema_version"`
	CardSchemaVersion string `json:"card_schema_version"`
	CompiledCaseCount int    `json:"compiled_case_count"`
	Checksum          string `json:"checksum"`
	Publisher         string `json:"publisher"`
	CreatedAt         string `json:"created_at"`
}

func HandlePublishFactoryPack(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxFactoryPackUploadBytes))
		if err != nil {
			writeFactoryError(w, http.StatusBadRequest, "read pack body failed")
			return
		}
		if len(body) == 0 {
			writeFactoryError(w, http.StatusBadRequest, "pack body is required")
			return
		}
		tmp, err := os.CreateTemp("", "factory-upload-*.pack")
		if err != nil {
			writeFactoryError(w, http.StatusInternalServerError, "create temp pack failed")
			return
		}
		tmpPath := tmp.Name()
		defer func() { _ = os.Remove(tmpPath) }()
		if _, err := tmp.Write(body); err != nil {
			_ = tmp.Close()
			writeFactoryError(w, http.StatusInternalServerError, "write temp pack failed")
			return
		}
		if err := tmp.Close(); err != nil {
			writeFactoryError(w, http.StatusInternalServerError, "close temp pack failed")
			return
		}
		loaded, err := pack.Load(tmpPath)
		if err != nil {
			writeFactoryError(w, http.StatusBadRequest, fmt.Sprintf("invalid factory pack: %v", err))
			return
		}
		created, err := store.CreateFactoryPackVersion(FactoryPackVersion{
			PackName:          loaded.Manifest.PackName,
			PackVersion:       loaded.Manifest.PackVersion,
			SchemaVersion:     loaded.Manifest.SchemaVersion,
			CardSchemaVersion: loaded.Manifest.CardSchemaVersion,
			Checksum:          loaded.Manifest.Checksum,
			CompiledCaseCount: loaded.Manifest.CompiledCaseCount,
			Publisher:         UserFromContext(r.Context()),
			Data:              body,
		})
		if err != nil {
			writeFactoryError(w, http.StatusInternalServerError, "store factory pack failed")
			return
		}
		writeFactoryJSON(w, http.StatusCreated, factoryPackMetadata(created))
	}
}

func HandleGetLatestFactoryPack(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := store.GetLatestFactoryPackVersion()
		if err != nil {
			if errors.Is(err, ErrFactoryPackNotFound) {
				writeFactoryError(w, http.StatusNotFound, "factory pack not found")
				return
			}
			writeFactoryError(w, http.StatusInternalServerError, "get latest factory pack failed")
			return
		}
		writeFactoryJSON(w, http.StatusOK, factoryPackMetadata(p))
	}
}

func HandleDownloadFactoryPack(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			writeFactoryError(w, http.StatusBadRequest, "invalid factory pack id")
			return
		}
		p, err := store.GetFactoryPackVersion(id)
		writeFactoryPackDownload(w, p, err)
	}
}

func HandleDownloadLatestFactoryPack(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := store.GetLatestFactoryPackVersion()
		writeFactoryPackDownload(w, p, err)
	}
}

func writeFactoryPackDownload(w http.ResponseWriter, p *FactoryPackVersion, err error) {
	if err != nil {
		if errors.Is(err, ErrFactoryPackNotFound) {
			writeFactoryError(w, http.StatusNotFound, "factory pack not found")
			return
		}
		writeFactoryError(w, http.StatusInternalServerError, "download factory pack failed")
		return
	}
	filename := filepath.Base(pack.FileName)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	_, _ = w.Write(p.Data)
}

func factoryPackMetadata(p *FactoryPackVersion) factoryPackMetadataResponse {
	return factoryPackMetadataResponse{
		ID:                p.ID,
		PackName:          p.PackName,
		PackVersion:       p.PackVersion,
		SchemaVersion:     p.SchemaVersion,
		CardSchemaVersion: p.CardSchemaVersion,
		CompiledCaseCount: p.CompiledCaseCount,
		Checksum:          p.Checksum,
		Publisher:         p.Publisher,
		CreatedAt:         p.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

func writeFactoryJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeFactoryError(w http.ResponseWriter, status int, message string) {
	writeFactoryJSON(w, status, map[string]string{"error": message})
}
