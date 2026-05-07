package pathpolicy

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mindspore-lab/mindspore-cli/tools"
)

func TestResolveReadablePathAllowsWorkspaceAndExternalReadRoot(t *testing.T) {
	workDir := t.TempDir()
	external := t.TempDir()
	resolver := NewResolver(NewPathPolicy(workDir, []string{external}, nil))

	got, denial, err := resolver.ResolveReadablePath("local.txt", ResolveOptions{})
	if err != nil || denial != nil {
		t.Fatalf("workspace readable err=%v denial=%v", err, denial)
	}
	if want := filepath.Join(workDir, "local.txt"); got != want {
		t.Fatalf("workspace path = %q, want %q", got, want)
	}

	got, denial, err = resolver.ResolveReadablePath(filepath.Join(external, "file.txt"), ResolveOptions{})
	if err != nil || denial != nil {
		t.Fatalf("external readable err=%v denial=%v", err, denial)
	}
	if want := filepath.Join(external, "file.txt"); got != want {
		t.Fatalf("external path = %q, want %q", got, want)
	}
}

func TestResolveReadablePathReturnsDenialForUnauthorizedExternalPath(t *testing.T) {
	workDir := t.TempDir()
	external := filepath.Join(t.TempDir(), "file.txt")
	resolver := NewResolver(NewPathPolicy(workDir, nil, nil))

	got, denial, err := resolver.ResolveReadablePath(external, ResolveOptions{})
	if err != nil {
		t.Fatalf("ResolveReadablePath error = %v", err)
	}
	if got != "" {
		t.Fatalf("resolved path = %q, want empty", got)
	}
	if denial == nil || denial.Kind != string(DenialKindExternalRead) {
		t.Fatalf("denial = %#v, want external read denial", denial)
	}
}

func TestResolveReadablePathAllowsTemporaryReadRootFromOptions(t *testing.T) {
	workDir := t.TempDir()
	external := t.TempDir()
	resolver := NewResolver(NewPathPolicy(workDir, nil, nil))

	got, denial, err := resolver.ResolveReadablePath(filepath.Join(external, "file.txt"), ResolveOptions{TemporaryReadRoots: []string{external}})
	if err != nil || denial != nil {
		t.Fatalf("temporary external readable err=%v denial=%v", err, denial)
	}
	if want := filepath.Join(external, "file.txt"); got != want {
		t.Fatalf("temporary external path = %q, want %q", got, want)
	}
}
func TestResolveWritablePathIsWorkspaceOnlyInM1(t *testing.T) {
	workDir := t.TempDir()
	external := t.TempDir()
	resolver := NewResolver(NewPathPolicy(workDir, []string{external}, nil))

	_, denial, err := resolver.ResolveWritablePath(filepath.Join(external, "file.txt"), ResolveOptions{})
	if err != nil {
		t.Fatalf("ResolveWritablePath error = %v", err)
	}
	if denial == nil || denial.Kind != string(DenialKindExternalWrite) {
		t.Fatalf("denial = %#v, want external write denial", denial)
	}
}

func TestExtractPathDenialSupportsPointerStructAndMap(t *testing.T) {
	denial := &PathDenial{Kind: string(DenialKindExternalRead), Operation: "read", InputPath: "/x"}
	got, ok := ExtractPathDenial(NewPathDenialResult(denial))
	if !ok || got.Kind != denial.Kind {
		t.Fatalf("ExtractPathDenial pointer got=%#v ok=%v", got, ok)
	}

	data, err := json.Marshal(denial)
	if err != nil {
		t.Fatal(err)
	}
	var asMap map[string]any
	if err := json.Unmarshal(data, &asMap); err != nil {
		t.Fatal(err)
	}
	got, ok = ExtractPathDenial(&tools.Result{Meta: map[string]any{PathDenialMetaKey: asMap}})
	if !ok || got.InputPath != denial.InputPath {
		t.Fatalf("ExtractPathDenial map got=%#v ok=%v", got, ok)
	}

	if got, ok := ExtractPathDenial(nil); ok || got != nil {
		t.Fatalf("ExtractPathDenial nil got=%#v ok=%v", got, ok)
	}
}
