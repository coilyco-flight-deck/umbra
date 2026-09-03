package execverb

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/urfave/cli/v3"
)

// umbra#6821: absence carries four meanings at once - withheld by policy, not
// implemented, not offered upstream, or the search missed it. A stub states one.
const withholdGuardfile = `wrap ward repo {
    exec gh
    can run list
    can run archive
    withhold delete {
        reason "Deleting a repo exceeds what this audit trail can reconstruct."
        alternative "archive"
    }
}`

func buildWithhold(t *testing.T, src string) (*cli.Command, error) {
	t.Helper()
	gf, err := Parse([]byte(src))
	if err != nil {
		return nil, err
	}
	return Build(Config{Guardfile: gf, Run: func(context.Context, string, []string, []string) error {
		t.Error("a withheld verb must reach no binary")
		return nil
	}})
}

func TestWithheldVerbIsVisibleInTheTree(t *testing.T) {
	root, err := buildWithhold(t, withholdGuardfile)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	leaf := findChild(root, "delete")
	if leaf == nil {
		t.Fatal("a withheld verb must be mounted: an absent one is the silence it replaces")
	}
	// The refusal leads, so a reader scanning the first clause rules it out.
	if !strings.HasPrefix(leaf.Usage, "NOT AVAILABLE") {
		t.Errorf("usage should lead with the refusal, got %q", leaf.Usage)
	}
	for _, want := range []string{"audit trail", "archive"} {
		if !strings.Contains(leaf.Usage, want) {
			t.Errorf("usage missing %q: %s", want, leaf.Usage)
		}
	}
}

func TestWithheldVerbRefusesAndSpawnsNothing(t *testing.T) {
	root, err := buildWithhold(t, withholdGuardfile)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	// The Run above fails the test if anything spawns.
	err = root.Run(context.Background(), []string{"repo", "delete", "kai/demo"})
	if err == nil {
		t.Fatal("a withheld verb must refuse every call")
	}
	var coded interface{ Kind() string }
	if !asCoded(err, &coded) || coded.Kind() != "policy_denied" {
		t.Errorf("kind = %v, want policy_denied: a refusal is not a failure to spawn", err)
	}
	for _, want := range []string{"withheld", "audit trail"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error missing %q: %v", want, err)
		}
	}
}

// A stub shadowing a live grant would advertise a working verb as refused, and
// the grant would look revoked with nothing revoking it.
func TestWithholdCannotShadowAGrantedVerb(t *testing.T) {
	src := `wrap ward repo {
    exec gh
    can run delete
    withhold delete {
        reason "x"
    }
}`
	if _, err := buildWithhold(t, src); err == nil {
		t.Fatal("a stub over a granted verb must fail closed")
	}
}

// A named alternative that does not exist sends a caller hunting for a verb
// they will never find, which is the second failure the stub exists to prevent.
func TestWithholdRefusesAnUngrantedAlternative(t *testing.T) {
	src := `wrap ward repo {
    exec gh
    can run list
    withhold delete {
        reason "x"
        alternative "archive"
    }
}`
	_, err := buildWithhold(t, src)
	if err == nil {
		t.Fatal("an alternative no grant mints must fail closed")
	}
	if !strings.Contains(err.Error(), "archive") {
		t.Errorf("error should name the missing alternative: %v", err)
	}
}

func TestWithholdNeedsAReason(t *testing.T) {
	src := `wrap ward repo {
    exec gh
    can run list
    withhold delete
}`
	if _, err := buildWithhold(t, src); err == nil {
		t.Fatal("a stub with no reason restates the absence it replaces and must fail closed")
	}
}

// A funnel takes the whole group, so a stub under one is unreachable. Refusing
// beats mounting something no caller can ever see.
func TestWithholdRefusedBesideAFunnel(t *testing.T) {
	for name, src := range map[string]string{
		"wildcard": `wrap ward repo {
    exec gh
    can run *
    withhold delete { reason "x" }
}`,
		"allow": `wrap "ward" "repo" {
    allow gh
    withhold delete { reason "x" }
}`,
	} {
		if _, err := buildWithhold(t, src); err == nil {
			t.Errorf("%s: a stub under a funnel is unreachable and must fail closed", name)
		}
	}
}

func TestWithholdRefusesUnknownChildAndDuplicate(t *testing.T) {
	cases := map[string]string{
		"unknown child": `wrap ward repo {
    exec gh
    can run list
    withhold delete { reason "x"; because "y" }
}`,
		"duplicate stub": `wrap ward repo {
    exec gh
    can run list
    withhold delete { reason "x" }
    withhold delete { reason "y" }
}`,
	}
	for name, src := range cases {
		if _, err := buildWithhold(t, src); err == nil {
			t.Errorf("%s must fail closed", name)
		}
	}
}

// asCoded is errors.As over the coded-error interface the exitcode package mints.
func asCoded(err error, target *interface{ Kind() string }) bool {
	return errors.As(err, target)
}
