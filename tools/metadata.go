package tools

var builtinToolMetadata = map[string]ToolMetadata{
	"read": {
		Classes: []ToolClass{ToolClassExploration, ToolClassVerification},
	},
	"grep": {
		Classes: []ToolClass{ToolClassExploration, ToolClassVerification},
	},
	"glob": {
		Classes: []ToolClass{ToolClassExploration},
	},
	"shell": {
		Classes: []ToolClass{ToolClassExploration, ToolClassVerification, ToolClassExternalEffect},
	},
	"write": {
		Classes: []ToolClass{ToolClassMutation},
	},
	"edit": {
		Classes: []ToolClass{ToolClassMutation},
	},
	"load_skill": {
		Classes: []ToolClass{ToolClassContextExpansion, ToolClassExploration},
	},
	"AskUserQuestion": {
		Classes: []ToolClass{ToolClassInteraction},
	},
}

func builtinMetadata(name string) ToolMetadata {
	metadata, ok := builtinToolMetadata[name]
	if !ok {
		return ToolMetadata{}
	}
	return cloneMetadata(metadata)
}

func cloneMetadata(metadata ToolMetadata) ToolMetadata {
	return ToolMetadata{Classes: append([]ToolClass(nil), metadata.Classes...)}
}
