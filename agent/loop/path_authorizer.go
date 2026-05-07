package loop

import (
	"context"

	"github.com/mindspore-lab/mindspore-cli/internal/pathpolicy"
)

type PathAuthorizationScope string

const (
	PathAuthorizationDeny       PathAuthorizationScope = "deny"
	PathAuthorizationOnce       PathAuthorizationScope = "once"
	PathAuthorizationSession    PathAuthorizationScope = "session"
	PathAuthorizationPersistent PathAuthorizationScope = "persistent"
)

type PathAuthorizationDecision struct {
	Scope PathAuthorizationScope
	Root  string
	Mode  string
}

type PathAuthorizer interface {
	RequestPathAuthorization(ctx context.Context, denial *pathpolicy.PathDenial) (PathAuthorizationDecision, error)
}
