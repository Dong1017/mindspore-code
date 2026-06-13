// Package local defines the interface for local-side readiness probes.
package local

import (
	"context"

	"gitcode.com/mindspore/mscli/internal/train"
	"gitcode.com/mindspore/mscli/runtime/probes"
)

// Probe checks local machine readiness before training.
type Probe interface {
	Run(ctx context.Context, req train.Request) ([]probes.Result, error)
}
