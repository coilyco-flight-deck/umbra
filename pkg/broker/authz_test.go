package broker

import (
	"context"
	"testing"
)

func TestPolicyAuthorize(t *testing.T) {
	cases := []struct {
		name   string
		policy Policy
		req    Request
		wantOK bool
	}{
		{
			name:   "file with title allowed",
			policy: Policy{Owners: []string{"acme"}, Ops: WriteOps},
			req:    Request{Op: OpFileIssue, Target: Target{Owner: "acme", Repo: "r"}, Title: "t"},
			wantOK: true,
		},
		{
			name:   "file without title refused",
			policy: Policy{Owners: []string{"acme"}, Ops: WriteOps},
			req:    Request{Op: OpFileIssue, Target: Target{Owner: "acme", Repo: "r"}},
			wantOK: false,
		},
		{
			name:   "edit needs positive number",
			policy: Policy{Owners: []string{"acme"}, Ops: WriteOps},
			req:    Request{Op: OpEditIssue, Target: Target{Owner: "acme", Repo: "r"}},
			wantOK: false,
		},
		{
			name:   "edit with number allowed",
			policy: Policy{Owners: []string{"acme"}, Ops: WriteOps},
			req:    Request{Op: OpEditIssue, Target: Target{Owner: "acme", Repo: "r", Number: 5}},
			wantOK: true,
		},
		{
			name:   "missing owner refused",
			policy: Policy{Owners: []string{"acme"}, Ops: WriteOps},
			req:    Request{Op: OpCommentIssue, Target: Target{Repo: "r", Number: 5}},
			wantOK: false,
		},
		{
			name:   "owner allowlist permits listed",
			policy: Policy{Owners: []string{"acme"}, Ops: WriteOps},
			req:    Request{Op: OpDispatch, Target: Target{Owner: "acme", Repo: "r", Number: 1}},
			wantOK: true,
		},
		{
			name:   "owner allowlist rejects unlisted",
			policy: Policy{Owners: []string{"acme"}, Ops: WriteOps},
			req:    Request{Op: OpDispatch, Target: Target{Owner: "evil", Repo: "r", Number: 1}},
			wantOK: false,
		},
		{
			name:   "op allowlist narrows surface",
			policy: Policy{Owners: []string{"acme"}, Ops: map[Op]bool{OpCommentIssue: true}},
			req:    Request{Op: OpDispatch, Target: Target{Owner: "acme", Repo: "r", Number: 1}},
			wantOK: false,
		},
		{
			name:   "unknown op refused",
			policy: Policy{Owners: []string{"acme"}, Ops: WriteOps},
			req:    Request{Op: Op("delete_everything"), Target: Target{Owner: "acme", Repo: "r", Number: 1}},
			wantOK: false,
		},
		{
			name:   "label add with number and labels allowed",
			policy: Policy{Owners: []string{"acme"}, Ops: WriteOps},
			req:    Request{Op: OpLabelIssue, Target: Target{Owner: "acme", Repo: "r", Number: 5}, LabelMode: LabelAdd, Labels: []string{"headless"}},
			wantOK: true,
		},
		{
			name:   "label set with labels allowed",
			policy: Policy{Owners: []string{"acme"}, Ops: WriteOps},
			req:    Request{Op: OpLabelIssue, Target: Target{Owner: "acme", Repo: "r", Number: 5}, LabelMode: LabelSet, Labels: []string{"a", "b"}},
			wantOK: true,
		},
		{
			name:   "label remove with labels allowed",
			policy: Policy{Owners: []string{"acme"}, Ops: WriteOps},
			req:    Request{Op: OpLabelIssue, Target: Target{Owner: "acme", Repo: "r", Number: 5}, LabelMode: LabelRemove, Labels: []string{"stale"}},
			wantOK: true,
		},
		{
			name:   "label without number refused",
			policy: Policy{Owners: []string{"acme"}, Ops: WriteOps},
			req:    Request{Op: OpLabelIssue, Target: Target{Owner: "acme", Repo: "r"}, LabelMode: LabelAdd, Labels: []string{"headless"}},
			wantOK: false,
		},
		{
			name:   "label without labels refused",
			policy: Policy{Owners: []string{"acme"}, Ops: WriteOps},
			req:    Request{Op: OpLabelIssue, Target: Target{Owner: "acme", Repo: "r", Number: 5}, LabelMode: LabelAdd},
			wantOK: false,
		},
		{
			name:   "label with unknown mode refused",
			policy: Policy{Owners: []string{"acme"}, Ops: WriteOps},
			req:    Request{Op: OpLabelIssue, Target: Target{Owner: "acme", Repo: "r", Number: 5}, LabelMode: "toggle", Labels: []string{"headless"}},
			wantOK: false,
		},
		{
			name:   "label with empty mode refused",
			policy: Policy{Owners: []string{"acme"}, Ops: WriteOps},
			req:    Request{Op: OpLabelIssue, Target: Target{Owner: "acme", Repo: "r", Number: 5}, Labels: []string{"headless"}},
			wantOK: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.policy.Authorize(context.Background(), tc.req)
			if tc.wantOK && err != nil {
				t.Fatalf("want allowed, got %v", err)
			}
			if !tc.wantOK && err == nil {
				t.Fatal("want refused, got nil")
			}
		})
	}
}

// The request below is well-formed and passes under a declared policy, so each
// refusal here is for the omission rather than for the request.
func TestPolicyFailsClosed(t *testing.T) {
	req := Request{Op: OpFileIssue, Target: Target{Owner: "acme", Repo: "r"}, Title: "t"}
	cases := []struct {
		name   string
		policy Policy
	}{
		{"zero value declares nothing", Policy{}},
		{"owners without ops", Policy{Owners: []string{"acme"}}},
		{"ops without owners", Policy{Ops: WriteOps}},
		{"empty ops map", Policy{Owners: []string{"acme"}, Ops: map[Op]bool{}}},
		{"empty owners slice", Policy{Owners: []string{}, Ops: WriteOps}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.policy.Validate(); err == nil {
				t.Error("Validate should refuse an under-declared policy")
			}
			if err := tc.policy.Authorize(context.Background(), req); err == nil {
				t.Fatal("want refused, got nil")
			}
		})
	}
}

// AnyOwner is the named opt-in: it is the only way an empty Owners permits
// anything, and it still refuses an empty owner.
func TestPolicyAnyOwner(t *testing.T) {
	p := Policy{AnyOwner: true, Ops: WriteOps}
	if err := p.Validate(); err != nil {
		t.Fatalf("AnyOwner should be a complete declaration: %v", err)
	}
	req := Request{Op: OpFileIssue, Target: Target{Owner: "whoever", Repo: "r"}, Title: "t"}
	if err := p.Authorize(context.Background(), req); err != nil {
		t.Fatalf("AnyOwner should admit an arbitrary owner: %v", err)
	}
	req.Target.Owner = ""
	if err := p.Authorize(context.Background(), req); err == nil {
		t.Fatal("AnyOwner must still refuse an empty owner")
	}
}

func TestPolicyValidateAcceptsDeclaredPolicy(t *testing.T) {
	if err := (Policy{Owners: []string{"acme"}, Ops: WriteOps}).Validate(); err != nil {
		t.Fatalf("want valid, got %v", err)
	}
}

func TestAuthorizerFunc(t *testing.T) {
	called := false
	var a Authorizer = AuthorizerFunc(func(context.Context, Request) error {
		called = true
		return nil
	})
	if err := a.Authorize(context.Background(), Request{}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !called {
		t.Fatal("AuthorizerFunc not invoked")
	}
}
