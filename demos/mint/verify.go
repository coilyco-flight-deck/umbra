package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

// Result is what verify and render leave in .render/<slug>/result.json, and
// the only thing the review sheet reads.
type Result struct {
	Slug     string       `json:"slug"`
	State    string       `json:"state"`
	Pass     bool         `json:"pass"`
	Failures []string     `json:"failures"`
	Steps    []StepResult `json:"steps"`
	Widest   int          `json:"widest"`
	Lines    int          `json:"lines"`
	Clip     string       `json:"clip,omitempty"`
	Frame    string       `json:"frame,omitempty"`
	Seconds  float64      `json:"seconds,omitempty"`
}

// StepResult is one step as it actually ran.
type StepResult struct {
	Cmd    string `json:"cmd"`
	Exit   int    `json:"exit"`
	Output string `json:"output"`
}

// workspace is one demo's scratch tree. fixture is the only directory the
// camera sees, so nothing of the harness leaks into a shot.
type workspace struct{ root, build, shims, fixture, home string }

func newWorkspace(demosRoot, slug string) (workspace, error) {
	root := filepath.Join(demosRoot, ".render", slug)
	w := workspace{root, filepath.Join(root, "build"), filepath.Join(root, "shims"), filepath.Join(root, "fixture"), filepath.Join(root, "home")}
	// Rename aside before deleting: a back-to-back rerun intermittently lost a
	// RemoveAll race inside the previous run's HOME, and a rename cannot.
	if _, err := os.Stat(root); err == nil {
		if err := os.Rename(root, fmt.Sprintf("%s.trash-%d", root, time.Now().UnixNano())); err != nil {
			return w, err
		}
	}
	if old, _ := filepath.Glob(root + ".trash-*"); len(old) > 0 {
		for _, o := range old {
			_ = os.RemoveAll(o)
		}
	}
	for _, d := range []string{w.build, w.shims, w.fixture, w.home} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return w, err
		}
	}
	return w, nil
}

// envFile is sourced by verify and the tape alike, so what was checked is what
// was filmed. HOME points inside the workspace, away from real credentials.
func (w workspace) envFile() string { return filepath.Join(w.root, "env.sh") }

func (w workspace) writeEnv() error {
	env := fmt.Sprintf(`export PATH=%q:"$PATH"
export HOME=%q
export PS1='$ '
export TERM=xterm-256color
export BASH_SILENCE_DEPRECATION_WARNING=1
export GIT_AUTHOR_NAME=demo GIT_AUTHOR_EMAIL=demo@example.invalid
export GIT_COMMITTER_NAME=demo GIT_COMMITTER_EMAIL=demo@example.invalid
export GIT_CONFIG_NOSYSTEM=1
cd %q
`, w.shims, w.home, w.fixture)
	return os.WriteFile(w.envFile(), []byte(env), 0o644)
}

func verify(demosRoot string, d *Demo) (*Result, workspace, error) {
	res := &Result{Slug: d.Slug, State: d.State}
	w, err := newWorkspace(demosRoot, d.Slug)
	if err != nil {
		return nil, w, err
	}
	if err := w.install(d); err != nil {
		return nil, w, err
	}
	if err := w.setup(d); err != nil {
		return nil, w, err
	}
	if err := w.writeEnv(); err != nil {
		return nil, w, err
	}
	// The prompt line of every step plus the trailing prompt.
	res.Lines = len(d.Steps) + 1
	for _, s := range d.Steps {
		out, code := w.run(s.Cmd)
		res.Steps = append(res.Steps, StepResult{Cmd: s.Cmd, Exit: code, Output: out})
		if code != s.Exit {
			res.Failures = append(res.Failures, fmt.Sprintf("%q exited %d, manifest says %d", s.Cmd, code, s.Exit))
		}
		if !strings.Contains(out, s.Shows) {
			res.Failures = append(res.Failures, fmt.Sprintf("%q output lacks %q", s.Cmd, s.Shows))
		}
		for _, l := range append(strings.Split(out, "\n"), "$ "+s.Cmd) {
			res.Widest = max(res.Widest, utf8.RuneCountInString(l))
		}
		if out != "" {
			res.Lines += strings.Count(out, "\n") + 1
		}
	}
	if res.Widest > d.Cols {
		res.Failures = append(res.Failures, fmt.Sprintf("widest line is %d columns, frame is %d: it would wrap on camera", res.Widest, d.Cols))
	}
	if res.Lines > d.Rows {
		res.Failures = append(res.Failures, fmt.Sprintf("transcript is %d lines, frame is %d: the first steps would scroll off", res.Lines, d.Rows))
	}
	res.Pass = len(res.Failures) == 0
	return res, w, writeResult(w, res)
}

func (w workspace) install(d *Demo) error {
	if err := os.CopyFS(filepath.Join(w.build, ".umbra"), os.DirFS(filepath.Join(d.Dir, ".umbra"))); err != nil {
		return fmt.Errorf("%s: copy .umbra: %w", d.Slug, err)
	}
	for _, args := range [][]string{{"lock"}, {"install", "--shim-dir", w.shims}} {
		cmd := exec.Command("umbra", args...)
		cmd.Dir = w.build
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: umbra %s: %w\n%s", d.Slug, args[0], err, out)
		}
	}
	if _, err := os.Stat(filepath.Join(w.shims, d.Tool)); err != nil {
		return fmt.Errorf("%s: umbra installed no %q shim; does the guardfile wrap `exec %s` with `replace`?", d.Slug, d.Tool, d.Tool)
	}
	return nil
}

func (w workspace) setup(d *Demo) error {
	for _, a := range d.Setup {
		var err error
		switch a.Kind {
		case "git-init":
			// The real git, before the shim is on PATH: setup is not on camera.
			cmd := exec.Command("git", "init", "-q", "-b", "main", ".")
			cmd.Dir = w.fixture
			err = cmd.Run()
		case "file":
			p := filepath.Join(w.fixture, a.Path)
			if err = os.MkdirAll(filepath.Dir(p), 0o755); err == nil {
				err = os.WriteFile(p, []byte(a.Body), 0o644)
			}
		case "guardfile":
			var src []string
			src, err = filepath.Glob(filepath.Join(d.Dir, ".umbra", "*.kdl"))
			if err == nil && len(src) != 1 {
				err = fmt.Errorf("`guardfile` setup needs exactly one .umbra/*.kdl, found %d", len(src))
			}
			if err == nil {
				var b []byte
				if b, err = os.ReadFile(src[0]); err == nil {
					err = os.WriteFile(filepath.Join(w.fixture, a.Path), b, 0o644)
				}
			}
		}
		if err != nil {
			return fmt.Errorf("%s: setup %s: %w", d.Slug, a.Kind, err)
		}
	}
	return nil
}

func (w workspace) run(cmdline string) (string, int) {
	cmd := exec.Command("bash", "--noprofile", "--norc", "-c", "source "+w.envFile()+"\n"+cmdline)
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	} else if err != nil {
		code = -1
	}
	return strings.TrimRight(buf.String(), "\n"), code
}

func writeResult(w workspace, r *Result) error {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(w.root, "result.json"), b, 0o644)
}
