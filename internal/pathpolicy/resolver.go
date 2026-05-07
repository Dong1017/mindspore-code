package pathpolicy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Resolver struct {
	policy *PathPolicy
}

type ResolveOptions struct {
	TemporaryReadRoots  []string
	TemporaryWriteRoots []string
}

type resolveOptionsContextKey struct{}

func ContextWithResolveOptions(ctx context.Context, opts ResolveOptions) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, resolveOptionsContextKey{}, opts)
}

func ResolveOptionsFromContext(ctx context.Context) ResolveOptions {
	if ctx == nil {
		return ResolveOptions{}
	}
	if opts, ok := ctx.Value(resolveOptionsContextKey{}).(ResolveOptions); ok {
		return opts
	}
	return ResolveOptions{}
}

func NewResolver(policy *PathPolicy) *Resolver {
	return &Resolver{policy: policy}
}

func (r *Resolver) WorkDir() string {
	if r == nil || r.policy == nil {
		return ""
	}
	return r.policy.WorkDir
}

func (r *Resolver) ResolveReadablePath(input string, opts ResolveOptions) (string, *PathDenial, error) {
	return r.resolvePath("read", string(DenialKindExternalRead), input, opts, true)
}

func (r *Resolver) ResolveReadablePathForOperation(operation, input string, opts ResolveOptions) (string, *PathDenial, error) {
	return r.resolvePath(operation, string(DenialKindExternalRead), input, opts, true)
}

func (r *Resolver) ResolveWritablePath(input string, opts ResolveOptions) (string, *PathDenial, error) {
	return r.resolvePath("write", string(DenialKindExternalWrite), input, opts, false)
}

func (r *Resolver) ResolveWritablePathForOperation(operation, input string, opts ResolveOptions) (string, *PathDenial, error) {
	return r.resolvePath(operation, string(DenialKindExternalWrite), input, opts, false)
}

func (r *Resolver) resolvePath(operation, denialKind, input string, opts ResolveOptions, readable bool) (string, *PathDenial, error) {
	if strings.TrimSpace(input) == "" {
		return "", nil, fmt.Errorf("path is required")
	}
	if r == nil || r.policy == nil {
		return "", nil, fmt.Errorf("path policy is not configured")
	}

	workDir, err := filepath.Abs(NormalizeInputPath(r.policy.WorkDir))
	if err != nil {
		return "", nil, fmt.Errorf("resolve working directory: %w", err)
	}
	cleaned := NormalizeInputPath(input)

	var fullAbs string
	if filepath.IsAbs(cleaned) {
		fullAbs, err = filepath.Abs(cleaned)
	} else {
		fullAbs, err = filepath.Abs(filepath.Join(workDir, cleaned))
	}
	if err != nil {
		return "", nil, fmt.Errorf("resolve path: %w", err)
	}

	if isIgnoredGitPath(fullAbs) {
		return "", nil, fmt.Errorf("path is ignored: %s", input)
	}
	if pathWithinBase(workDir, fullAbs) {
		return fullAbs, nil, nil
	}
	if !filepath.IsAbs(cleaned) {
		return "", nil, fmt.Errorf("path escapes working directory: %s", input)
	}

	if readable && r.allowedByRoots(fullAbs, opts.TemporaryReadRoots, true) {
		return fullAbs, nil, nil
	}
	if !readable && r.allowedByRoots(fullAbs, opts.TemporaryWriteRoots, false) {
		return fullAbs, nil, nil
	}

	suggestion := SuggestRoot(fullAbs)
	reason := "The path is outside the current workspace and is not in external_read_roots."
	if !readable {
		reason = "The path is outside the current workspace; external write roots are not enabled in this release."
	}
	return "", &PathDenial{
		Kind:          denialKind,
		Operation:     operation,
		InputPath:     input,
		ResolvedPath:  fullAbs,
		WorkDir:       workDir,
		SuggestedRoot: suggestion.Path,
		RootKind:      suggestion.Kind,
		Reason:        reason,
	}, nil
}

func (r *Resolver) allowedByRoots(target string, temporaryRoots []string, readable bool) bool {
	for _, root := range temporaryRoots {
		if rootAllows(root, target) {
			return true
		}
	}
	if r == nil || r.policy == nil {
		return false
	}
	if readable {
		for _, entry := range r.policy.ReadRoots.Snapshot() {
			if rootAllows(entry.Path, target) {
				return true
			}
		}
		for _, entry := range r.policy.WriteRoots.Snapshot() {
			if rootAllows(entry.Path, target) {
				return true
			}
		}
		return false
	}
	for _, entry := range r.policy.WriteRoots.Snapshot() {
		if rootAllows(entry.Path, target) {
			return true
		}
	}
	return false
}

func rootAllows(root, target string) bool {
	cleanedRoot := NormalizeInputPath(root)
	if strings.TrimSpace(cleanedRoot) == "" {
		return false
	}
	rootAbs, err := filepath.Abs(cleanedRoot)
	if err != nil {
		return false
	}
	return pathWithinBase(rootAbs, target)
}

func pathWithinBase(base, target string) bool {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func isIgnoredGitPath(path string) bool {
	cleaned := filepath.ToSlash(filepath.Clean(path))
	for _, part := range strings.Split(cleaned, "/") {
		if part == ".git" {
			return true
		}
	}
	return false
}

func IsIgnoredGitName(name string) bool {
	return name == ".git"
}
