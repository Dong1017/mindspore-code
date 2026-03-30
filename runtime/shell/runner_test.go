package shell

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRunnerRunStream_EmitsOutputWhileCollectingResult(t *testing.T) {
	runner := NewRunner(Config{
		WorkDir: ".",
		Timeout: 2 * time.Second,
	})

	var (
		mu     sync.Mutex
		chunks []OutputChunk
	)
	result, err := runner.RunStream(context.Background(), "printf 'out-1\\nout-2\\n'; printf 'err-1\\n' >&2", func(chunk OutputChunk) {
		mu.Lock()
		chunks = append(chunks, chunk)
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("RunStream failed: %v", err)
	}

	if got, want := result.Stdout, "out-1\nout-2"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if got, want := result.Stderr, "err-1"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}

	var sawStdout, sawStderr bool
	for _, chunk := range chunks {
		if chunk.Stream == StreamStdout && strings.Contains(chunk.Text, "out-1") {
			sawStdout = true
		}
		if chunk.Stream == StreamStderr && strings.Contains(chunk.Text, "err-1") {
			sawStderr = true
		}
	}
	if !sawStdout {
		t.Fatalf("expected streamed stdout chunk, got %#v", chunks)
	}
	if !sawStderr {
		t.Fatalf("expected streamed stderr chunk, got %#v", chunks)
	}
}
