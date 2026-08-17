package broker

import (
	"context"
	"errors"
	"fmt"
)

// Authorizer vets each [Request] before execution - the write-tier check. It
// returns nil to permit and a non-nil error to refuse (folded into [Response]).
type Authorizer interface {
	Authorize(ctx context.Context, req Request) error
}

// AuthorizerFunc adapts a plain function to the [Authorizer] interface.
type AuthorizerFunc func(ctx context.Context, req Request) error

// Authorize calls f.
func (f AuthorizerFunc) Authorize(ctx context.Context, req Request) error {
	return f(ctx, req)
}

// Policy is the default [Authorizer]: owner allowlist x op allowlist, plus the
// structural invariants every op needs. It fails closed - see docs/broker.md.
type Policy struct {
	// Owners is the allowlist of issue owners. Empty denies every request
	// unless AnyOwner is set.
	Owners []string
	// AnyOwner is the named opt-in accepting any non-empty owner, and the only
	// way an empty Owners permits anything.
	AnyOwner bool
	// Ops is the allowlist of permitted operations. Empty denies every
	// operation; pass [WriteOps] for the full write tier.
	Ops map[Op]bool
}

// Validate reports whether the policy declares enough to permit anything, so a
// consumer can fail at startup rather than at its first refused request.
func (p Policy) Validate() error {
	if len(p.Ops) == 0 {
		return errors.New("broker: policy permits no operations (set Ops, e.g. to WriteOps)")
	}
	if len(p.Owners) == 0 && !p.AnyOwner {
		return errors.New("broker: policy allows no owners (set Owners, or AnyOwner to accept every owner)")
	}
	return nil
}

// Authorize enforces the policy. ctx is accepted for interface conformance;
// the built-in checks are synchronous.
func (p Policy) Authorize(_ context.Context, req Request) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if !p.Ops[req.Op] {
		return fmt.Errorf("broker: operation %q not permitted", req.Op)
	}
	if req.Target.Owner == "" || req.Target.Repo == "" {
		return fmt.Errorf("broker: %s requires target owner and repo", req.Op)
	}
	if !p.ownerAllowed(req.Target.Owner) {
		return fmt.Errorf("broker: owner %q not in allowlist", req.Target.Owner)
	}
	// Every op but file-issue acts on an existing issue and needs its number.
	if req.Op != OpFileIssue && req.Target.Number <= 0 {
		return fmt.Errorf("broker: %s requires a positive issue number", req.Op)
	}
	if req.Op == OpFileIssue && req.Title == "" {
		return fmt.Errorf("broker: %s requires a title", req.Op)
	}
	if req.Op == OpLabelIssue {
		if err := labelInvariants(req); err != nil {
			return err
		}
	}
	return nil
}

// labelInvariants enforces the [OpLabelIssue] rules fail-closed: a known mode
// ([LabelAdd]/[LabelSet]/[LabelRemove]) and at least one label to act on.
func labelInvariants(req Request) error {
	switch req.LabelMode {
	case LabelAdd, LabelSet, LabelRemove:
	default:
		return fmt.Errorf("broker: %s has unknown mode %q", req.Op, req.LabelMode)
	}
	if len(req.Labels) == 0 {
		return fmt.Errorf("broker: %s %s requires at least one label", req.Op, req.LabelMode)
	}
	return nil
}

// ownerAllowed applies the Owners allowlist. An empty owner is never allowed,
// including under AnyOwner.
func (p Policy) ownerAllowed(owner string) bool {
	if owner == "" {
		return false
	}
	if p.AnyOwner {
		return true
	}
	for _, o := range p.Owners {
		if o == owner {
			return true
		}
	}
	return false
}
