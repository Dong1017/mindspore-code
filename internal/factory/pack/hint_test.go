package pack_test

import (
	"strings"
	"testing"

	"github.com/mindspore-lab/mindspore-cli/internal/factory/pack"
)

func TestRenderFactoryHintBlock(t *testing.T) {
	block, err := pack.RenderFactoryHintBlock([]pack.CaseMatch{{
		CaseID:               "stable-ascend-import",
		Title:                "Ascend import runtime failure",
		ConfidenceLevel:      "observed",
		Score:                99,
		WhyMatched:           []string{"keyword matched: torch_npu"},
		SuggestedNextChecks:  []string{"Check CANN environment variables."},
		SuggestedFixTemplate: "Source CANN environment before import.",
		Verification:         []string{"Run python import smoke test."},
		MissingEvidence:      []string{"driver version is unknown"},
		ConflictingSignals:   []string{"logs are incomplete"},
	}})
	if err != nil {
		t.Fatalf("RenderFactoryHintBlock() error = %v", err)
	}
	assertContains(t, block, "[Factory Diagnostic Hints]")
	assertContains(t, block, "why_matched")
	assertContains(t, block, "keyword matched: torch_npu")
	assertNotContains(t, block, "Score")
	assertNotContains(t, block, "score")
}

func TestRenderFactoryHintBlockExcludesRawContent(t *testing.T) {
	longLog := strings.Repeat("full-log-line ", 300)
	block, err := pack.RenderFactoryHintBlock([]pack.CaseMatch{{
		CaseID:               "raw-check",
		Title:                "Raw content check",
		ConfidenceLevel:      "observed",
		Score:                88,
		WhyMatched:           []string{"id: raw\nkind: known_issue\ntitle: raw yaml", "CREATE TABLE cases(id TEXT); INSERT INTO manifest VALUES('checksum', 'sha256:abc')"},
		SuggestedNextChecks:  []string{"inspect bounded evidence only " + longLog},
		SuggestedFixTemplate: "do not expose raw sqlite pattern_type rows",
		Verification:         []string{"{case_id: raw-check, pattern_type: regex}"},
	}})
	if err != nil {
		t.Fatalf("RenderFactoryHintBlock() error = %v", err)
	}
	assertNotContains(t, block, "kind: known_issue")
	assertNotContains(t, block, "\nkind:")
	assertNotContains(t, block, "pattern_type")
	assertNotContains(t, block, "CREATE TABLE")
	assertNotContains(t, block, "INSERT INTO")
	if strings.Count(block, "full-log-line") > 40 {
		t.Fatalf("hint block contains %d full-log-line tokens, want bounded output", strings.Count(block, "full-log-line"))
	}
	assertNotContains(t, block, "score")
}

func TestRenderFactoryHintBlockBudgetPreservesLineStructure(t *testing.T) {
	longText := strings.Repeat("word ", 1200)
	matches := []pack.CaseMatch{
		{CaseID: "one", Title: longText, ConfidenceLevel: "observed", WhyMatched: []string{longText}},
		{CaseID: "two", Title: longText, ConfidenceLevel: "observed", WhyMatched: []string{longText}},
		{CaseID: "three", Title: longText, ConfidenceLevel: "observed", WhyMatched: []string{longText}},
		{CaseID: "four", Title: longText, ConfidenceLevel: "observed", WhyMatched: []string{longText}},
	}
	block, err := pack.RenderFactoryHintBlock(matches)
	if err != nil {
		t.Fatalf("RenderFactoryHintBlock() error = %v", err)
	}
	if got := len(strings.Fields(block)); got > pack.MaxHintBlockTokens {
		t.Fatalf("hint block tokens = %d, want <= %d", got, pack.MaxHintBlockTokens)
	}
	assertContains(t, block, "[Factory Diagnostic Hints]\n")
	assertContains(t, block, "\n[/Factory Diagnostic Hints]")
	assertContains(t, block, "Hint 1:\n")
	assertContains(t, block, "- case_id: one\n")
	assertNotContains(t, block, "case_id: four")
}

func assertContains(t *testing.T, value, want string) {
	t.Helper()
	if !strings.Contains(value, want) {
		t.Fatalf("expected %q to contain %q", value, want)
	}
}

func assertNotContains(t *testing.T, value, forbidden string) {
	t.Helper()
	if strings.Contains(value, forbidden) {
		t.Fatalf("expected %q not to contain %q", value, forbidden)
	}
}
