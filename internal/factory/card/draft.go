package card

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	factoryruntime "github.com/mindspore-lab/mindspore-cli/internal/factory/runtime"
	"gopkg.in/yaml.v3"
)

const DefaultDraftCardsDir = "factory/cards/drafts"

type DraftOptions struct {
	Reporter string
	HomeDir  string
}

func NewDraftFromRunSummary(summary factoryruntime.LastRunSummary, opts DraftOptions) (*KnownIssueCard, error) {
	topic := draftTopic(summary)
	if strings.TrimSpace(topic) == "" {
		return nil, fmt.Errorf("last run summary is empty")
	}
	problemType, needsReview := draftProblemType(summary)
	stage := draftStage(summary)
	symptoms := draftSymptoms(summary)
	keywords := draftKeywords(summary)
	if len(symptoms) == 0 {
		symptoms = []string{topic}
	}
	if len(keywords) == 0 {
		keywords = []string{topic}
	}
	card := &KnownIssueCard{
		SchemaVersion: SchemaVersionKnownIssueV05,
		Kind:          KindKnownIssue,
		ID:            draftCardID(topic),
		Title:         trimWords(topic, 18),
		Tags:          []string{"factory-draft"},
		Case: Case{
			ProblemType: problemType,
			Stage:       stage,
			Domain:      draftDomain(summary),
			Hardware:    draftHardware(summary),
			Severity:    SeverityUnknown,
			Environment: Environment{
				Frameworks: draftFrameworks(summary),
			},
		},
		Match: Match{
			Keywords: keywords,
		},
		Guidance: Guidance{
			Symptom:              symptoms[0],
			TriggerSignals:       keywords,
			RepresentativeErrors: draftRepresentativeErrors(summary),
			Diagnosis:            "Draft generated from the latest bounded run summary; review and complete before promotion.",
			DiagnosisDetails:     draftDiagnosisDetails(needsReview, summary.Warning),
			Fix:                  draftFixSummary(summary),
			Verification:         draftVerification(summary),
		},
		Provenance: Provenance{
			Notes: draftProvenanceNotes(needsReview, summary.Warning),
		},
		Governance: Governance{
			Confidence:   ConfidenceBootstrap,
			Lifecycle:    LifecycleDraft,
			ReviewStatus: ReviewPending,
			Rationale:    "Generated from a bounded local run summary and requires human review.",
			UpdatedAt:    time.Now().UTC().Format(time.RFC3339),
		},
	}
	if strings.TrimSpace(opts.Reporter) != "" {
		card.Provenance.Notes = strings.TrimSpace(card.Provenance.Notes + " draft reporter: " + strings.TrimSpace(opts.Reporter))
	}
	if err := SanitizeDraftCard(card, opts); err != nil {
		return nil, err
	}
	if err := ValidateDraft(card); err != nil {
		return nil, err
	}
	return card, nil
}

func WriteDraftYAML(card *KnownIssueCard, outputDir string) (string, error) {
	if strings.TrimSpace(outputDir) == "" {
		outputDir = DefaultDraftCardsDir
	}
	if err := os.MkdirAll(outputDir, 0o700); err != nil {
		return "", fmt.Errorf("create draft card dir: %w", err)
	}
	path := filepath.Join(outputDir, card.ID+".yaml")
	data, err := yaml.Marshal(card)
	if err != nil {
		return "", fmt.Errorf("render draft card yaml: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return "", fmt.Errorf("draft card already exists: %s", path)
		}
		return "", fmt.Errorf("write draft card: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return "", fmt.Errorf("write draft card: %w", err)
	}
	return path, nil
}

func draftTopic(summary factoryruntime.LastRunSummary) string {
	if summary.Fix != nil {
		return firstDraftText(summary.Fix.Topic, summary.Fix.UserProblemSummary, summary.Fix.PlannedFixSummary)
	}
	if summary.Diagnose != nil {
		return firstDraftText(summary.Diagnose.Topic, summary.Diagnose.UserProblemSummary)
	}
	return ""
}

func draftProblemType(summary factoryruntime.LastRunSummary) (string, bool) {
	text := strings.ToLower(joinSummaryText(summary))
	if strings.Contains(text, ProblemTypeAccuracy) || strings.Contains(text, "loss") || strings.Contains(text, "nan") || strings.Contains(text, "precision") {
		return ProblemTypeAccuracy, false
	}
	if strings.Contains(text, ProblemTypePerformance) || strings.Contains(text, "throughput") || strings.Contains(text, "latency") || strings.Contains(text, "slow") {
		return ProblemTypePerformance, false
	}
	if strings.Contains(text, ProblemTypeFailure) || strings.Contains(text, "error") || strings.Contains(text, "exception") || strings.Contains(text, "failed") || strings.Contains(text, "crash") || strings.Contains(text, "oom") {
		return ProblemTypeFailure, false
	}
	return ProblemTypeUnknown, true
}

func draftStage(summary factoryruntime.LastRunSummary) string {
	text := strings.ToLower(joinSummaryText(summary))
	stages := []struct {
		stage   string
		signals []string
	}{
		{StageImport, []string{"importerror", "import ", "module not found", "modulenotfounderror"}},
		{StageCompile, []string{"compile", "graph compile", "build graph"}},
		{StageTrain, []string{"train", "training", "loss", "backward"}},
		{StageEval, []string{"eval", "evaluation", "validation"}},
		{StageInfer, []string{"infer", "inference", "predict"}},
		{StageData, []string{"dataset", "dataloader", "data loader"}},
		{StageSetup, []string{"install", "setup", "environment", "env var"}},
	}
	for _, candidate := range stages {
		for _, signal := range candidate.signals {
			if strings.Contains(text, signal) {
				return candidate.stage
			}
		}
	}
	return StageUnknown
}

func draftDomain(summary factoryruntime.LastRunSummary) string {
	text := strings.ToLower(joinSummaryText(summary))
	for _, domain := range []string{DomainMindSpore, DomainTorchNPU, DomainTorch, DomainCANN} {
		if strings.Contains(text, domain) {
			return domain
		}
	}
	return DomainUnknown
}

func draftHardware(summary factoryruntime.LastRunSummary) string {
	text := strings.ToLower(joinSummaryText(summary))
	if strings.Contains(text, "ascend") || strings.Contains(text, "npu") || strings.Contains(text, "cann") || strings.Contains(text, "acl") {
		return HardwareAscend
	}
	if strings.Contains(text, "cuda") || strings.Contains(text, "gpu") {
		return HardwareGPU
	}
	if strings.Contains(text, "cpu") {
		return HardwareCPU
	}
	return HardwareUnknown
}

func draftSymptoms(summary factoryruntime.LastRunSummary) []string {
	values := summaryEvidence(summary)
	values = append(values, draftTopic(summary))
	return boundedUnique(values, 6)
}

func draftKeywords(summary factoryruntime.LastRunSummary) []string {
	return boundedUnique(summaryEvidence(summary), 8)
}

func draftFrameworks(summary factoryruntime.LastRunSummary) []Framework {
	text := strings.ToLower(joinSummaryText(summary))
	var frameworks []Framework
	for _, name := range []string{FrameworkMindSpore, FrameworkTorch, FrameworkTorchNPU} {
		if strings.Contains(text, name) {
			frameworks = append(frameworks, Framework{Name: name})
		}
	}
	return frameworks
}

func draftFixSummary(summary factoryruntime.LastRunSummary) string {
	if summary.Fix != nil {
		return summary.Fix.PlannedFixSummary
	}
	return ""
}

func draftVerification(summary factoryruntime.LastRunSummary) string {
	if summary.Fix == nil || len(summary.Fix.Verification) == 0 {
		return "not verified; reviewer must add validation steps"
	}
	return strings.Join(boundedUnique(summary.Fix.Verification, 4), "\n")
}

func draftRepresentativeErrors(summary factoryruntime.LastRunSummary) []string {
	var out []string
	for _, value := range summaryEvidence(summary) {
		lower := strings.ToLower(value)
		if strings.Contains(lower, "error") || strings.Contains(lower, "exception") || strings.Contains(lower, "traceback") || strings.Contains(lower, "failed") {
			out = append(out, value)
		}
	}
	return boundedUnique(out, 4)
}

func draftDiagnosisDetails(needsReview bool, warning string) []string {
	parts := []string{"Draft card requires human review before stable promotion."}
	if needsReview {
		parts = append(parts, "Problem type was unclear from the run summary and was set to unknown.")
	}
	if strings.TrimSpace(warning) != "" {
		parts = append(parts, warning)
	}
	return parts
}

func draftProvenanceNotes(needsReview bool, warning string) string {
	parts := []string{"Generated from latest bounded in-memory run summary; no raw logs or full command output included."}
	if needsReview {
		parts = append(parts, "Needs review: unclear problem type set to unknown.")
	}
	if strings.TrimSpace(warning) != "" {
		parts = append(parts, "Warning: "+warning)
	}
	return strings.Join(parts, " ")
}

func summaryEvidence(summary factoryruntime.LastRunSummary) []string {
	var values []string
	if summary.Diagnose != nil {
		values = append(values, summary.Diagnose.KeyEvidence...)
		for _, hint := range summary.Diagnose.FactoryHintsUsed {
			values = append(values, hint.Title)
			values = append(values, hint.WhyMatched...)
		}
	}
	if summary.Fix != nil {
		values = append(values, summary.Fix.KeyEvidence...)
	}
	return values
}

func joinSummaryText(summary factoryruntime.LastRunSummary) string {
	var values []string
	if summary.Diagnose != nil {
		values = append(values, summary.Diagnose.Topic, summary.Diagnose.UserProblemSummary)
		values = append(values, summary.Diagnose.KeyEvidence...)
	}
	if summary.Fix != nil {
		values = append(values, summary.Fix.Topic, summary.Fix.UserProblemSummary, summary.Fix.PlannedFixSummary)
		values = append(values, summary.Fix.KeyEvidence...)
	}
	return strings.Join(values, " ")
}

func firstDraftText(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func draftCardID(topic string) string {
	slug := slugify(topic)
	if slug == "" {
		slug = "factory-draft"
	}
	h := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(topic))))
	return fmt.Sprintf("draft-%s-%s", slug, hex.EncodeToString(h[:])[:8])
}

func slugify(value string) string {
	value = strings.ToLower(value)
	re := regexp.MustCompile(`[^a-z0-9]+`)
	value = re.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	parts := strings.Split(value, "-")
	if len(parts) > 6 {
		parts = parts[:6]
	}
	return strings.Join(parts, "-")
}

func boundedUnique(values []string, limit int) []string {
	out := make([]string, 0, limit)
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = trimWords(value, 24)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func trimWords(value string, limit int) string {
	fields := strings.Fields(strings.TrimSpace(value))
	if len(fields) <= limit {
		return strings.Join(fields, " ")
	}
	return strings.Join(fields[:limit], " ")
}
