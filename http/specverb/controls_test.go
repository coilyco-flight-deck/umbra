package specverb

import (
	"context"
	"testing"
)

func TestSpecNegativeControlsHold(t *testing.T) {
	gf, spec := denyFixture(t)
	cs, err := NegativeControls(context.Background(), gf, spec)
	if err != nil {
		t.Fatalf("NegativeControls: %v", err)
	}
	if len(cs) != 2 {
		t.Fatalf("got %d controls, want one per never grant: %+v", len(cs), cs)
	}
	for _, c := range cs {
		if !c.Holds() {
			t.Errorf("%s %q does not hold: observed %s, without it %s", c.Kind, c.Rule, c.Observed, c.Without)
		}
	}
}
