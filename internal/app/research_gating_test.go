package app

import "testing"

func TestResearchDisabledContinuationIntent(t *testing.T) {
	for _, input := range []string{"continue", "go on", "proceed", "keep going", "go ahead", "please summarize", "final answer"} {
		if !isResearchDisabledContinuationIntent(input) {
			t.Fatalf("isResearchDisabledContinuationIntent(%q) = false, want true", input)
		}
	}
	for _, input := range []string{"inspect more files", "fix the bug", "run tests"} {
		if isResearchDisabledContinuationIntent(input) {
			t.Fatalf("isResearchDisabledContinuationIntent(%q) = true, want false", input)
		}
	}
}
