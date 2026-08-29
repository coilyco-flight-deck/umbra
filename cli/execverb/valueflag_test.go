package execverb

import "testing"

// agentic-os#1351: the built-in value-flag table is AWS-shaped, so a kubectl
// grant guarding argN read `--context ser8`'s value as a positional.
func TestPositionalsHonorsDeclaredValueFlags(t *testing.T) {
	argv := []string{"get", "pods", "--context", "ser8"}
	// Undeclared, the flag's value lands in the positional list, so an `arg2`
	// guard reads "ser8" believing it read a resource name.
	if got := positionals(argv); len(got) != 3 || got[2] != "ser8" {
		t.Fatalf("baseline changed; got %v", got)
	}
	got := positionals(argv, "--context")
	if len(got) != 2 || got[0] != "get" || got[1] != "pods" {
		t.Fatalf("declared value-flag not consumed; got %v", got)
	}
}

func TestArgNGuardOnUndeclaredFlagIsRefusedAtBuild(t *testing.T) {
	g := Grant{
		Subcommand: []string{"get"},
		AllowFlags: []string{"--context"},
		Whens:      []WhenClause{{Selector: "arg0", Patterns: []string{"pods"}}},
	}
	if err := g.validateShape(); err == nil {
		t.Fatal("an argN guard beside an unknown-arity flag must be refused")
	}
	g.ValueFlags = []string{"--context"}
	if err := g.validateShape(); err != nil {
		t.Fatalf("declaring the flag should satisfy it: %v", err)
	}
}

func TestFlagSelectorGuardsStayUnaffected(t *testing.T) {
	g := Grant{
		Subcommand: []string{"get"},
		AllowFlags: []string{"--context"},
		Whens:      []WhenClause{{Selector: "context", Patterns: []string{"ser8"}}},
	}
	if err := g.validateShape(); err != nil {
		t.Fatalf("a flag selector reads argv directly and needs no declaration: %v", err)
	}
}
