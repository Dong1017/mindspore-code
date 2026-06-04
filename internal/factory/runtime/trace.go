package runtime

type FactoryEnrichmentTrace struct {
	PackLoadStatus   string
	ManifestSummary  string
	CandidateCount   int
	EmittedHintCount int
	FallbackReason   string
}
