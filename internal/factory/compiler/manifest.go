package compiler

import (
	"fmt"
	"strconv"
	"time"

	"github.com/mindspore-lab/mindspore-cli/internal/factory/pack"
)

func buildManifest(summary *BuildSummary, sourceHash string) map[string]string {
	checksum := summary.Checksum
	if checksum == "" {
		checksum = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	}
	return map[string]string{
		pack.ManifestKeyPackName:          summary.PackName,
		pack.ManifestKeyPackVersion:       time.Now().UTC().Format("2006.01.02"),
		pack.ManifestKeySchemaVersion:     summary.SchemaVersion,
		pack.ManifestKeyBuildTime:         time.Now().UTC().Format(time.RFC3339),
		pack.ManifestKeySourceCaseCount:   strconv.Itoa(summary.SourceCaseCount),
		pack.ManifestKeyCompiledCaseCount: strconv.Itoa(summary.CompiledCaseCount),
		pack.ManifestKeySourceCommitHash:  sourceHash,
		pack.ManifestKeyMinMSCLIVersion:   pack.MinMSCLIVer,
		pack.ManifestKeyChecksum:          checksum,
	}
}

func requireManifestFields(manifest map[string]string) error {
	for _, key := range pack.RequiredManifestKeys {
		if manifest[key] == "" {
			return fmt.Errorf("manifest missing required field: %s", key)
		}
	}
	return nil
}
