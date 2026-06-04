package compiler

import "github.com/mindspore-lab/mindspore-cli/internal/factory/pack"

type BuildSummary struct {
	PackName           string
	OutputPath         string
	SchemaVersion      string
	SourceCaseCount    int
	CompiledCaseCount  int
	DraftExcluded      int
	DeprecatedExcluded int
	ArchivedExcluded   int
	InvalidCount       int
	Checksum           string
}

func newBuildSummary(outputPath string) *BuildSummary {
	return &BuildSummary{
		PackName:      pack.Name,
		OutputPath:    outputPath,
		SchemaVersion: pack.SchemaVersion,
	}
}
