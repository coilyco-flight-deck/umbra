// Package negcontrol is the shared result of a negative control, which the
// dialects produce and the umbra driver reports. See docs/negative-controls.md.
package negcontrol

import (
	"context"
	"fmt"
	"io"

	"github.com/urfave/cli/v3"
)

// Control is one rule's result.
type Control struct {
	Kind        string // "never", "withhold", or "spec never"
	Rule        string // the refused path, e.g. "config set"
	Argv        []string
	Refused     bool   // the shipped guardfile refused with this rule's own text
	LoadBearing bool   // removing the rule changed the observed outcome
	Observed    string // what the shipped guardfile did
	Without     string // what it did with the rule removed
}

// Holds reports whether the control passes: refused by its own text, and
// removing it would let something different happen.
func (c Control) Holds() bool { return c.Refused && c.LoadBearing }

// Outcome is what one invocation did, observed rather than read off the tree.
type Outcome struct {
	Spawned bool
	Code    int
	Text    string
}

func (o Outcome) String() string {
	if o.Spawned {
		return "reached the upstream"
	}
	if o.Code == 0 && o.Text == "" {
		return "exited 0 without spawning"
	}
	return fmt.Sprintf("exit %d: %s", o.Code, o.Text)
}

// Judge builds a Control from the two observed outcomes. want is the rule's own
// refusal text: exit 2 alone would pass on a refusal that came from elsewhere.
func Judge(kind, rule string, argv []string, got, without Outcome, want string, denied int) Control {
	return Control{
		Kind:        kind,
		Rule:        rule,
		Argv:        argv,
		Refused:     !got.Spawned && got.Code == denied && got.Text == want,
		LoadBearing: without != got,
		Observed:    got.String(),
		Without:     without.String(),
	}
}

// Summarize counts passing controls and names every failure, so a run
// cannot report a pass while omitting a rule it could not hold.
func Summarize(cs []Control) (held int, failed []string) {
	for _, c := range cs {
		if c.Holds() {
			held++
			continue
		}
		why := "not refused by its own text"
		if c.Refused {
			why = "removing it changes nothing"
		}
		failed = append(failed, fmt.Sprintf("%s %q: %s (observed %s, without it %s)",
			c.Kind, c.Rule, why, c.Observed, c.Without))
	}
	return held, failed
}

// Run runs argv on root in-process with urfave's exit globals swapped out.
// Not safe to call concurrently. See docs/negative-controls.md.
func Run(ctx context.Context, root *cli.Command, argv []string) error {
	exiter, errw := cli.OsExiter, cli.ErrWriter
	cli.OsExiter, cli.ErrWriter = func(int) {}, io.Discard
	defer func() { cli.OsExiter, cli.ErrWriter = exiter, errw }()
	quiet(root)
	return root.Run(ctx, argv)
}

func quiet(root *cli.Command) {
	root.Writer, root.ErrWriter = io.Discard, io.Discard
	root.ExitErrHandler = func(context.Context, *cli.Command, error) {}
	for _, c := range root.Commands {
		quiet(c)
	}
}
