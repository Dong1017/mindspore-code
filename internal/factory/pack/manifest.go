package pack

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

type Manifest struct {
	PackName           string
	PackVersion        string
	SchemaVersion      string
	BuildTime          string
	SourceCaseCount    int
	CompiledCaseCount  int
	SourceCommitOrHash string
	MinMSCLIVersion    string
	Checksum           string
}

var checksumPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func parseManifest(values map[string]string) (*Manifest, error) {
	for _, key := range RequiredManifestKeys {
		if values[key] == "" {
			return nil, fmt.Errorf("manifest missing required field: %s", key)
		}
	}

	sourceCaseCount, err := parseManifestInt(values, ManifestKeySourceCaseCount)
	if err != nil {
		return nil, err
	}
	compiledCaseCount, err := parseManifestInt(values, ManifestKeyCompiledCaseCount)
	if err != nil {
		return nil, err
	}

	manifest := &Manifest{
		PackName:           values[ManifestKeyPackName],
		PackVersion:        values[ManifestKeyPackVersion],
		SchemaVersion:      values[ManifestKeySchemaVersion],
		BuildTime:          values[ManifestKeyBuildTime],
		SourceCaseCount:    sourceCaseCount,
		CompiledCaseCount:  compiledCaseCount,
		SourceCommitOrHash: values[ManifestKeySourceCommitHash],
		MinMSCLIVersion:    values[ManifestKeyMinMSCLIVersion],
		Checksum:           values[ManifestKeyChecksum],
	}
	if err := validateManifest(manifest); err != nil {
		return nil, err
	}
	return manifest, nil
}

func validateManifest(manifest *Manifest) error {
	if manifest.PackName != Name {
		return fmt.Errorf("manifest pack_name must be %s", Name)
	}
	if manifest.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported pack schema_version: %s", manifest.SchemaVersion)
	}
	if _, err := time.Parse(time.RFC3339, manifest.BuildTime); err != nil {
		return fmt.Errorf("invalid manifest build_time: %w", err)
	}
	if manifest.SourceCaseCount < 0 {
		return fmt.Errorf("manifest source_case_count cannot be negative")
	}
	if manifest.CompiledCaseCount < 0 {
		return fmt.Errorf("manifest compiled_case_count cannot be negative")
	}
	if manifest.SourceCaseCount < manifest.CompiledCaseCount {
		return fmt.Errorf("manifest source_case_count cannot be less than compiled_case_count")
	}
	if !checksumPattern.MatchString(manifest.Checksum) {
		return fmt.Errorf("invalid manifest checksum format")
	}
	return nil
}

func parseManifestInt(values map[string]string, key string) (int, error) {
	value, err := strconv.Atoi(values[key])
	if err != nil {
		return 0, fmt.Errorf("invalid manifest %s: %w", key, err)
	}
	return value, nil
}
