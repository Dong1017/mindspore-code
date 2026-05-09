package runtime

import "testing"

func TestSelectLastRunSummaryMergesClearTopicMatch(t *testing.T) {
	diagnose := &DiagnoseRunSummary{Topic: "ImportError torch_npu missing"}
	fix := &FixRunSummary{Topic: "ImportError torch_npu missing"}
	selected := SelectLastRunSummary(diagnose, fix)
	if selected.Diagnose == nil || selected.Fix == nil {
		t.Fatalf("selected = %#v, want diagnose and fix", selected)
	}
	if selected.Warning != "" {
		t.Fatalf("Warning = %q, want empty", selected.Warning)
	}
}

func TestSelectLastRunSummaryUsesLatestFixWhenTopicUnclear(t *testing.T) {
	diagnose := &DiagnoseRunSummary{Topic: "accuracy loss"}
	fix := &FixRunSummary{Topic: "ImportError torch_npu missing"}
	selected := SelectLastRunSummary(diagnose, fix)
	if selected.Diagnose != nil || selected.Fix != fix {
		t.Fatalf("selected = %#v, want fix only", selected)
	}
	if selected.Warning == "" {
		t.Fatal("Warning is empty, want warning")
	}
}

func TestSelectLastRunSummaryUsesLatestRunKindWhenTopicsDiffer(t *testing.T) {
	diagnose := &DiagnoseRunSummary{Topic: "accuracy loss"}
	fix := &FixRunSummary{Topic: "ImportError torch_npu missing"}

	selected := SelectLastRunSummary(diagnose, fix, "diagnose")
	if selected.Diagnose != diagnose || selected.Fix != nil {
		t.Fatalf("selected = %#v, want diagnose only", selected)
	}
	if selected.Warning == "" {
		t.Fatal("Warning is empty, want warning")
	}

	selected = SelectLastRunSummary(diagnose, fix, "fix")
	if selected.Diagnose != nil || selected.Fix != fix {
		t.Fatalf("selected = %#v, want fix only", selected)
	}
	if selected.Warning == "" {
		t.Fatal("Warning is empty, want warning")
	}
}

func TestBuildFixRunSummaryBoundsAndDoesNotVerify(t *testing.T) {
	long := ""
	for i := 0; i < 400; i++ {
		long += "word "
	}
	summary := BuildFixRunSummary(FixRunSummaryInput{
		Topic:              long,
		UserProblemSummary: long,
		PlannedFixSummary:  long,
		KeyEvidence:        []string{"one", "two", "three", "four", "five", "six", "seven", "eight", "nine"},
	})
	if summary.Command != "fix" {
		t.Fatalf("Command = %q, want fix", summary.Command)
	}
	if len(summary.KeyEvidence) != 8 {
		t.Fatalf("KeyEvidence = %d, want 8", len(summary.KeyEvidence))
	}
	if len(summary.Verification) != 0 {
		t.Fatalf("Verification = %#v, want empty", summary.Verification)
	}
}
