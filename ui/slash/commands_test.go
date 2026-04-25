package slash

import "testing"

func TestDefaultRegistryIncludesBranchAndFork(t *testing.T) {
	registry := NewRegistry()

	for _, name := range []string{"/branch", "/fork"} {
		cmd, ok := registry.Get(name)
		if !ok {
			t.Fatalf("default registry missing %s", name)
		}
		if cmd.Hidden {
			t.Fatalf("%s should be visible in slash suggestions", name)
		}
	}
}
