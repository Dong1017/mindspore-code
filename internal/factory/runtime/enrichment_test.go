package runtime_test

import (
	"path/filepath"
	"strings"
	"testing"

	"gitcode.com/mindspore/mscli/internal/factory/compiler"
	"gitcode.com/mindspore/mscli/internal/factory/pack"
	"gitcode.com/mindspore/mscli/internal/factory/runtime"
)

func TestBuildFactoryHintBlockEmitsHint(t *testing.T) {
	path := buildRuntimeTestPack(t)
	ctx := knownRuntimeDiagnosticContext()

	hint, trace, err := runtime.BuildFactoryHintBlock(ctx, pack.LoadConfig{PackPath: path})
	if err != nil {
		t.Fatalf("BuildFactoryHintBlock() error = %v", err)
	}
	if !strings.Contains(hint, "[Factory Diagnostic Hints]") {
		t.Fatalf("hint = %q, want factory hint block", hint)
	}
	if !strings.Contains(hint, "stable-ascend-import") {
		t.Fatalf("hint = %q, want stable-ascend-import", hint)
	}
	if trace.PackLoadStatus != "loaded" {
		t.Fatalf("PackLoadStatus = %q, want loaded", trace.PackLoadStatus)
	}
	if trace.CandidateCount == 0 {
		t.Fatalf("CandidateCount = 0, want matches")
	}
	if trace.EmittedHintCount == 0 || trace.EmittedHintCount > pack.MaxHintCases {
		t.Fatalf("EmittedHintCount = %d, want 1..%d", trace.EmittedHintCount, pack.MaxHintCases)
	}
	if trace.FallbackReason != "" {
		t.Fatalf("FallbackReason = %q, want empty", trace.FallbackReason)
	}
}

func TestBuildFactoryHintBlockMissingPackFallsBackQuietly(t *testing.T) {
	hint, trace, err := runtime.BuildFactoryHintBlock(knownRuntimeDiagnosticContext(), pack.LoadConfig{PackPath: filepath.Join(t.TempDir(), pack.FileName)})
	if err != nil {
		t.Fatalf("BuildFactoryHintBlock() error = %v", err)
	}
	if hint != "" {
		t.Fatalf("hint = %q, want empty", hint)
	}
	if trace.PackLoadStatus != "failed" {
		t.Fatalf("PackLoadStatus = %q, want failed", trace.PackLoadStatus)
	}
	if !strings.Contains(trace.FallbackReason, "pack load failed") {
		t.Fatalf("FallbackReason = %q, want pack load failed", trace.FallbackReason)
	}
}

func TestBuildFactoryHintBlockNoMatchFallsBackQuietly(t *testing.T) {
	path := buildRuntimeTestPack(t)
	ctx := pack.DiagnosticContext{
		Command:   "/diagnose",
		UserInput: "The dataset labels are imbalanced and validation accuracy drifts slowly.",
		Problem: pack.DiagnosticProblem{
			InferredType:  "accuracy",
			InferredStage: "data",
		},
	}

	hint, trace, err := runtime.BuildFactoryHintBlock(ctx, pack.LoadConfig{PackPath: path})
	if err != nil {
		t.Fatalf("BuildFactoryHintBlock() error = %v", err)
	}
	if hint != "" {
		t.Fatalf("hint = %q, want empty", hint)
	}
	if trace.PackLoadStatus != "loaded" {
		t.Fatalf("PackLoadStatus = %q, want loaded", trace.PackLoadStatus)
	}
	if trace.FallbackReason != "no match" {
		t.Fatalf("FallbackReason = %q, want no match", trace.FallbackReason)
	}
}

func TestBuildDiagnoseRunSummaryBoundsData(t *testing.T) {
	ctx := knownRuntimeDiagnosticContext()
	ctx.UserInput = strings.Repeat("problem ", 300)
	ctx.Signals.Keywords = []string{"k1", "k2", "k3", "k4", "k5", "k6", "k7", "k8", "k9"}
	matches := []pack.CaseMatch{
		{CaseID: "1", Title: "one", WhyMatched: []string{"a", "b", "c", "d"}},
		{CaseID: "2", Title: "two"},
		{CaseID: "3", Title: "three"},
		{CaseID: "4", Title: "four"},
	}

	summary := runtime.BuildDiagnoseRunSummary(ctx, matches)
	if summary.Command != "diagnose" {
		t.Fatalf("Command = %q, want diagnose", summary.Command)
	}
	if len(summary.KeyEvidence) > 8 {
		t.Fatalf("KeyEvidence len = %d, want <= 8", len(summary.KeyEvidence))
	}
	if len(summary.FactoryHintsUsed) != 3 {
		t.Fatalf("FactoryHintsUsed len = %d, want 3", len(summary.FactoryHintsUsed))
	}
	if len(summary.FactoryHintsUsed[0].WhyMatched) != 3 {
		t.Fatalf("WhyMatched len = %d, want 3", len(summary.FactoryHintsUsed[0].WhyMatched))
	}
	if got := len(strings.Fields(summary.UserProblemSummary)); got > 240 {
		t.Fatalf("UserProblemSummary tokens = %d, want <= 240", got)
	}
	if summary.Privacy.RawLogsIncluded || summary.Privacy.SensitiveEnvIncluded {
		t.Fatalf("Privacy = %+v, want no raw logs or sensitive env", summary.Privacy)
	}
}

func buildRuntimeTestPack(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), pack.FileName)
	if _, err := compiler.CompilePack(filepath.FromSlash("../compiler/testdata/cards"), path); err != nil {
		t.Fatalf("CompilePack() error = %v", err)
	}
	return path
}

func knownRuntimeDiagnosticContext() pack.DiagnosticContext {
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
	}
}
