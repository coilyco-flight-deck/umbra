package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	kdl "github.com/calico32/kdl-go"
)

// States a demo moves through. Review is the only step that needs a human, so
// it is the only transition the harness never makes on its own.
var states = []string{"minted", "approved", "bounced", "published"}

// allFormats are the guardfile syntaxes umbra reads. A demo ships its policy
// in each one, and verify proves they refuse identically.
var allFormats = []string{"kdl", "yaml", "toml"}

// Demo is one parsed demo.kdl. Every field is structured on purpose: the only
// strings a manifest carries are commands, file bodies and expected substrings.
type Demo struct {
	Slug    string
	Dir     string
	Tool    string
	State   string
	Cols    int
	Rows    int
	Formats []string // the first is the one filmed
	Setup   []SetupAction
	Steps   []Step
}

// Guardfile is the demo's policy in one format.
func (d *Demo) Guardfile(format string) string {
	return filepath.Join(d.Dir, ".umbra", d.Tool+".guardfile."+format)
}

// SetupAction prepares the fixture directory the steps run in.
type SetupAction struct {
	Kind string // git-init, file, guardfile
	Path string
	Body string
}

// Step is one command typed on camera and the refusal (or success) it must show.
type Step struct {
	Cmd   string
	Exit  int
	Shows string
}

// Target is one backlog.kdl row: a demo that should exist.
type Target struct {
	Slug string
	Tool string
}

func loadDemo(dir string) (*Demo, error) {
	src, err := os.ReadFile(filepath.Join(dir, "demo.kdl"))
	if err != nil {
		return nil, err
	}
	doc, err := kdl.ParseString(string(src))
	if err != nil {
		return nil, fmt.Errorf("%s: parse KDL: %w", dir, err)
	}
	root := doc.GetNode("demo")
	if root == nil || len(root.Arguments()) != 1 {
		return nil, fmt.Errorf("%s: needs exactly one top-level `demo <slug>` node", dir)
	}
	d := &Demo{Slug: root.Arg(0).String(), Dir: dir, Cols: 80, Rows: 16, Formats: allFormats}
	if d.Slug != filepath.Base(dir) {
		return nil, fmt.Errorf("%s: demo slug %q must match its directory", dir, d.Slug)
	}
	for _, n := range root.Children().Nodes {
		if err := d.apply(n); err != nil {
			return nil, fmt.Errorf("%s: %w", dir, err)
		}
	}
	return d, d.validate()
}

func (d *Demo) apply(n *kdl.Node) error {
	switch n.Name() {
	case "tool":
		d.Tool = n.Arg(0).String()
	case "state":
		d.State = n.Arg(0).String()
	case "formats":
		d.Formats = nil
		for _, a := range n.Arguments() {
			d.Formats = append(d.Formats, a.String())
		}
	case "frame":
		if v := n.Prop("cols"); v.IsValid() {
			d.Cols = v.Int()
		}
		if v := n.Prop("rows"); v.IsValid() {
			d.Rows = v.Int()
		}
	case "setup":
		for _, c := range n.Children().Nodes {
			a := SetupAction{Kind: c.Name()}
			switch a.Kind {
			case "git-init":
			case "file":
				if len(c.Arguments()) != 2 {
					return fmt.Errorf("`file` takes a path and a body")
				}
				a.Path, a.Body = c.Arg(0).String(), c.Arg(1).String()
			case "guardfile":
				if len(c.Arguments()) != 1 {
					return fmt.Errorf("`guardfile` takes the fixture path to copy the guardfile to")
				}
				a.Path = c.Arg(0).String()
			default:
				return fmt.Errorf("unknown setup action %q (git-init, file, guardfile)", a.Kind)
			}
			d.Setup = append(d.Setup, a)
		}
	case "step":
		if len(n.Arguments()) != 1 || !n.Prop("exit").IsValid() || !n.Prop("shows").IsValid() {
			return fmt.Errorf("`step` needs a command, exit=<code> and shows=<substring>")
		}
		d.Steps = append(d.Steps, Step{Cmd: n.Arg(0).String(), Exit: n.Prop("exit").Int(), Shows: n.Prop("shows").String()})
	default:
		return fmt.Errorf("unknown node %q (tool, state, formats, frame, setup, step)", n.Name())
	}
	return nil
}

func (d *Demo) validate() error {
	if d.Tool == "" {
		return fmt.Errorf("%s: missing `tool`", d.Slug)
	}
	if !contains(states, d.State) {
		return fmt.Errorf("%s: state %q is not one of %s", d.Slug, d.State, strings.Join(states, ", "))
	}
	if len(d.Formats) == 0 {
		return fmt.Errorf("%s: `formats` names at least one of %s", d.Slug, strings.Join(allFormats, ", "))
	}
	for _, f := range d.Formats {
		if !contains(allFormats, f) {
			return fmt.Errorf("%s: format %q is not one of %s", d.Slug, f, strings.Join(allFormats, ", "))
		}
		if _, err := os.Stat(d.Guardfile(f)); err != nil {
			return fmt.Errorf("%s: format %s has no guardfile at .umbra/%s", d.Slug, f, filepath.Base(d.Guardfile(f)))
		}
	}
	if len(d.Steps) == 0 {
		return fmt.Errorf("%s: a demo needs at least one step", d.Slug)
	}
	for _, s := range d.Steps {
		if strings.Contains(s.Cmd, `"`) && strings.Contains(s.Cmd, "`") {
			return fmt.Errorf("%s: step %q mixes \" and `, which a tape cannot type", d.Slug, s.Cmd)
		}
	}
	return nil
}

func loadBacklog(root string) ([]Target, error) {
	src, err := os.ReadFile(filepath.Join(root, "backlog.kdl"))
	if err != nil {
		return nil, err
	}
	doc, err := kdl.ParseString(string(src))
	if err != nil {
		return nil, fmt.Errorf("backlog.kdl: %w", err)
	}
	var out []Target
	for _, n := range doc.Nodes {
		if n.Name() != "target" || len(n.Arguments()) != 1 || !n.Prop("tool").IsValid() {
			return nil, fmt.Errorf("backlog.kdl: every node is `target <slug> tool=<binary>`")
		}
		out = append(out, Target{Slug: n.Arg(0).String(), Tool: n.Prop("tool").String()})
	}
	return out, nil
}

// minted lists every directory under root that holds a demo.kdl.
func minted(root string) ([]string, error) {
	m, err := filepath.Glob(filepath.Join(root, "*", "demo.kdl"))
	if err != nil {
		return nil, err
	}
	var out []string
	for _, p := range m {
		out = append(out, filepath.Base(filepath.Dir(p)))
	}
	sort.Strings(out)
	return out, nil
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
