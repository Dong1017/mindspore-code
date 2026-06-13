// Package target defines the interface for remote target readiness probes.
package target

import (
	"context"

	"gitcode.com/mindspore/mscli/internal/train"
	"gitcode.com/mindspore/mscli/runtime/probes"
)

// Probe checks remote training target readiness.
type Probe interface {
	Run(ctx context.Context, target train.TrainTarget) ([]probes.Result, error)
}
