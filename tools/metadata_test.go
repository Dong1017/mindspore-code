package tools

import "testing"

func TestBuiltinToolMetadata(t *testing.T) {
	cases := []struct {
		name string
		want []ToolClass
	}{
		{name: "read", want: []ToolClass{ToolClassExploration, ToolClassVerification}},
		{name: "grep", want: []ToolClass{ToolClassExploration, ToolClassVerification}},
		{name: "glob", want: []ToolClass{ToolClassExploration}},
		{name: "shell", want: []ToolClass{ToolClassExploration, ToolClassVerification, ToolClassExternalEffect}},
		{name: "write", want: []ToolClass{ToolClassMutation}},
		{name: "edit", want: []ToolClass{ToolClassMutation}},
		{name: "load_skill", want: []ToolClass{ToolClassContextExpansion, ToolClassExploration}},
		{name: "AskUserQuestion", want: []ToolClass{ToolClassInteraction}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			metadata := builtinMetadata(tc.name)
			if !sameClasses(metadata.Classes, tc.want) {
				t.Fatalf("builtinMetadata(%q).Classes = %v, want %v", tc.name, metadata.Classes, tc.want)
			}
			metadata.Classes[0] = ToolClassMutation
			if !sameClasses(builtinMetadata(tc.name).Classes, tc.want) {
				t.Fatalf("mutating returned metadata changed builtin metadata for %q", tc.name)
			}
		})
	}
}

func TestBuiltinToolMetadataUnknownIsEmpty(t *testing.T) {
	if got := builtinMetadata("unknown").Classes; len(got) != 0 {
		t.Fatalf("builtinMetadata(unknown).Classes = %v, want empty", got)
	}
}
