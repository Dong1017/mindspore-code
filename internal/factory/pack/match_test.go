package pack_test

import (
	"path/filepath"
	"testing"

	"github.com/mindspore-lab/mindspore-cli/internal/factory/compiler"
	"github.com/mindspore-lab/mindspore-cli/internal/factory/pack"
)

func TestMatchCasesKnownError(t *testing.T) {
	loaded := loadMatchPack(t)
	matches, err := loaded.MatchCases(knownImportFingerprint(), pack.MatchOptions{})
	if err != nil {
		t.Fatalf("MatchCases() error = %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("expected matches")
	}
	if matches[0].CaseID != "stable-ascend-import" {
		t.Fatalf("top match = %q, want stable-ascend-import", matches[0].CaseID)
	}
	if matches[0].Score < pack.DefaultMinimumScoreToEmit {
		t.Fatalf("score = %d, want >= %d", matches[0].Score, pack.DefaultMinimumScoreToEmit)
	}
	if len(matches[0].WhyMatched) == 0 {
		t.Fatalf("WhyMatched is empty")
	}
}

func TestMatchCasesUnrelatedErrorReturnsNoMatch(t *testing.T) {
	loaded := loadMatchPack(t)
	fp := pack.DiagnosticContext{
		Command:   "/diagnose",
		UserInput: "The dataset labels are imbalanced and validation accuracy drifts slowly.",
		Problem: pack.DiagnosticProblem{
			InferredType:  "accuracy",
			InferredStage: "data",
		},
	}.ToFingerprint()
	matches, err := loaded.MatchCases(fp, pack.MatchOptions{})
	if err != nil {
		t.Fatalf("MatchCases() error = %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("matches = %v, want none", matches)
	}
}

func TestMatchCasesHardConflictFiltersCandidate(t *testing.T) {
	loaded := loadMatchPack(t)
	fp := knownImportFingerprint()
	fp.Signals.NegativeSignals = []string{"cuda-only"}
	matches, err := loaded.MatchCases(fp, pack.MatchOptions{MinimumScoreToEmit: 1})
	if err != nil {
		t.Fatalf("MatchCases() error = %v", err)
	}
	for _, match := range matches {
		if match.CaseID == "stable-ascend-import" {
			t.Fatalf("hard-conflicted candidate was emitted")
		}
	}
}

func TestMatchCasesWeakConflictDowngradesCandidate(t *testing.T) {
	loaded := loadMatchPack(t)
	base, err := loaded.MatchCases(knownImportFingerprint(), pack.MatchOptions{})
	if err != nil {
		t.Fatalf("base MatchCases() error = %v", err)
	}
	weak := knownImportFingerprint()
	weak.Signals.LogTail = "cuda-only appears in a partial environment note, but torch_npu import still fails"
	weakMatches, err := loaded.MatchCases(weak, pack.MatchOptions{})
	if err != nil {
		t.Fatalf("weak MatchCases() error = %v", err)
	}
	baseMatch := findMatch(base, "stable-ascend-import")
	weakMatch := findMatch(weakMatches, "stable-ascend-import")
	if baseMatch == nil || weakMatch == nil {
		t.Fatalf("expected stable-ascend-import in both result sets")
	}
	if weakMatch.Score >= baseMatch.Score {
		t.Fatalf("weak conflict score = %d, base score = %d", weakMatch.Score, baseMatch.Score)
	}
	if len(weakMatch.ConflictingSignals) == 0 {
		t.Fatalf("weak conflict did not record conflicting signal")
	}
}

func TestMatchCasesMissingMetadataDoesNotBlock(t *testing.T) {
	loaded := loadMatchPack(t)
	fp := knownImportFingerprint()
	fp.Stage = "unknown"
	fp.ProblemType = "unknown"
	fp.Environment.Hardware.Accelerator = "unknown"
	matches, err := loaded.MatchCases(fp, pack.MatchOptions{})
	if err != nil {
		t.Fatalf("MatchCases() error = %v", err)
	}
	if findMatch(matches, "stable-ascend-import") == nil {
		t.Fatalf("missing metadata blocked known text match")
	}
}

func TestMatchCasesBootstrapRanksLower(t *testing.T) {
	loaded := loadMatchPack(t)
	matches, err := loaded.MatchCases(knownImportFingerprint(), pack.MatchOptions{})
	if err != nil {
		t.Fatalf("MatchCases() error = %v", err)
	}
	stable := findMatch(matches, "stable-ascend-import")
	bootstrap := findMatch(matches, "bootstrap-import")
	if stable == nil || bootstrap == nil {
		t.Fatalf("expected stable and bootstrap matches: %v", matches)
	}
	if bootstrap.Score >= stable.Score {
		t.Fatalf("bootstrap score = %d, stable score = %d", bootstrap.Score, stable.Score)
	}
}

func TestMatchCasesBelowMinimumNotEmitted(t *testing.T) {
	loaded := loadMatchPack(t)
	matches, err := loaded.MatchCases(knownImportFingerprint(), pack.MatchOptions{MinimumScoreToEmit: 100})
	if err != nil {
		t.Fatalf("MatchCases() error = %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("matches = %v, want none", matches)
	}
}

func TestMatchCasesDefaultTopK(t *testing.T) {
	loaded := loadMatchPack(t)
	matches, err := loaded.MatchCases(knownImportFingerprint(), pack.MatchOptions{MinimumScoreToEmit: 1})
	if err != nil {
		t.Fatalf("MatchCases() error = %v", err)
	}
	if len(matches) > pack.DefaultTopK {
		t.Fatalf("matches len = %d, want <= %d", len(matches), pack.DefaultTopK)
	}
}

func loadMatchPack(t *testing.T) *pack.Pack {
	t.Helper()
	path := filepath.Join(t.TempDir(), pack.FileName)
	if _, err := compiler.CompilePack(filepath.FromSlash("../compiler/testdata/cards"), path); err != nil {
		t.Fatalf("CompilePack() error = %v", err)
	}
	loaded, err := pack.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return loaded
}

func knownImportFingerprint() pack.Fingerprint {
	return pack.DiagnosticContext{
		Command:   "/diagnose",
		UserInput: "ImportError: torch_npu failed because CANN runtime dependency is missing",
		Problem: pack.DiagnosticProblem{
			InferredType:  "failure",
			InferredStage: "import",
		},
		Signals: pack.DiagnosticSignals{
			MainError:     "ImportError: torch_npu runtime dependency missing",
			TracebackTail: "File train.py import torch_npu",
			Keywords:      []string{"torch_npu", "runtime dependency"},
		},
		Environment: pack.DiagnosticEnvironment{
			Hardware: pack.DiagnosticHardware{Accelerator: "ascend"},
			Runtime:  pack.DiagnosticRuntime{CANNVersion: "8.0"},
			Frameworks: []pack.DiagnosticFramework{
				{Name: "torch_npu", Version: "2.7.1.post2"},
			},
		},
	}.ToFingerprint()
}

func findMatch(matches []pack.CaseMatch, id string) *pack.CaseMatch {
	for i := range matches {
		if matches[i].CaseID == id {
			return &matches[i]
		}
	}
	return nil
}
