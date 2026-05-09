package pack

const (
	Name          = "factory-core"
	FileName      = "factory-core.pack"
	SchemaVersion = "1"
	MinMSCLIVer   = "0.1.0"
)

const (
	DefaultTopK               = 3
	DefaultMinimumScoreToEmit = 5
	MaxHintCases              = 3
	MaxHintBlockTokens        = 800
	MaxSingleCaseTokens       = 200
)

const (
	ManifestKeyPackName          = "pack_name"
	ManifestKeyPackVersion       = "pack_version"
	ManifestKeySchemaVersion     = "schema_version"
	ManifestKeyBuildTime         = "build_time"
	ManifestKeySourceCaseCount   = "source_case_count"
	ManifestKeyCompiledCaseCount = "compiled_case_count"
	ManifestKeySourceCommitHash  = "source_commit_or_hash"
	ManifestKeyMinMSCLIVersion   = "min_ms_cli_version"
	ManifestKeyChecksum          = "checksum"
)

var RequiredManifestKeys = []string{
	ManifestKeyPackName,
	ManifestKeyPackVersion,
	ManifestKeySchemaVersion,
	ManifestKeyBuildTime,
	ManifestKeySourceCaseCount,
	ManifestKeyCompiledCaseCount,
	ManifestKeySourceCommitHash,
	ManifestKeyMinMSCLIVersion,
	ManifestKeyChecksum,
}
