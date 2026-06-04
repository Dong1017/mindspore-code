package app

import (
	"testing"

	"github.com/mindspore-lab/mindspore-cli/configs"
)

func TestInitToolsRegisteredToolsHaveMetadata(t *testing.T) {
	registry := initTools(configs.DefaultConfig(), t.TempDir())
	for _, name := range registry.Names() {
		if got := registry.Metadata(name).Classes; len(got) == 0 {
			t.Fatalf("registered tool %q has no runtime metadata", name)
		}
	}
}
