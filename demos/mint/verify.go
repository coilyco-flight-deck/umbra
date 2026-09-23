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
	"sync"
	"time"
	"unicode/utf8"
)

// Result is what verify and render leave in .render/<slug>/result.json, and
// the only thing the review sheet reads.
type Result struct {
	Slug       string       `json:"slug"`
	State      string       `json:"state"`
	Pass       bool         `json:"pass"`
	Failures   []string     `json:"failures"`
	Formats    []string     `json:"formats"`
	Equivalent bool         `json:"equivalent"`
	Steps      []StepResult `json:"steps"`
	Widest     int          `json:"widest"`
	Lines      int          `json:"lines"`
	Clip       string       `json:"clip,omitempty"`
	Frame      string       `json:"frame,omitempty"`
	Seconds    float64      `json:"seconds,omitempty"`
}

// StepResult is one step as it actually ran.
type StepResult struct {
	Cmd    string `json:"cmd"`
	Exit   int    `json:"exit"`
	Output string `json:"output"`
}

// workspace is one demo's scratch tree. Each format gets its own build, shims,
// fixture and HOME, and fixture is the only directory the camera sees.
type workspace struct{ root string }

func (w workspace) dir(kind, format string) string { return filepath.Join(w.root, format, kind) }

// envFile is sourced by verify and the tape alike, so what was checked is what
// was filmed. HOME points inside the workspace, away from real credentials.
func (w workspace) envFile(format string) string { return filepath.Join(w.root, format, "env.sh") }

func newWorkspace(demosRoot, slug string) (workspace, error) {
	w := workspace{filepath.Join(demosRoot, ".render", slug)}
	// Rename aside before deleting: a back-to-back rerun intermittently lost a
	// RemoveAll race inside the previous run's HOME, and a rename cannot.
	if _, err := os.Stat(w.root); err == nil {
		if err := os.Rename(w.root, fmt.Sprintf("%s.trash-%d", w.root, time.Now().UnixNano())); err != nil {
			return w, err
		}
	}
	if old, _ := filepath.Glob(w.root + ".trash-*"); len(old) > 0 {
		for _, o := range old {
			_ = os.RemoveAll(o)
		}
	}
	return w, os.MkdirAll(w.root, 0o755)
}

func (w workspace) writeEnv(format string) error {
	env := fmt.Sprintf(`export PATH=%q:"$PATH"
export HOME=%q
export PS1='$ '
export TERM=xterm-256color
export BASH_SILENCE_DEPRECATION_WARNING=1
export GIT_AUTHOR_NAME=demo GIT_AUTHOR_EMAIL=demo@example.invalid
export GIT_COMMITTER_NAME=demo GIT_COMMITTER_EMAIL=demo@example.invalid
export GIT_CONFIG_NOSYSTEM=1
cd %q
`, w.dir("shims", format), w.dir("home", format), w.dir("fixture", format))
	return os.WriteFile(w.envFile(format), []byte(env), 0o644)
}

var (
	umbraOnce sync.Once
	umbraBin  string
	umbraErr  error
)

// buildUmbra compiles the driver from the checkout demos/ sits in, once per
// run. A demo tests the umbra beside it, never whichever one is installed.
func buildUmbra(demosRoot string) (string, error) {
	umbraOnce.Do(func() {
		umbraBin = filepath.Join(demosRoot, ".render", "bin", "umbra")
		cmd := exec.Command("go", "build", "-o", umbraBin, "./cmd/umbra")
		cmd.Dir = filepath.Dir(demosRoot)
		if out, err := cmd.CombinedOutput(); err != nil {
			umbraErr = fmt.Errorf("build umbra from %s: %w\n%s", cmd.Dir, err, out)
		}
	})
	return umbraBin, umbraErr
}

func verify(demosRoot string, d *Demo) (*Result, workspace, error) {
	res := &Result{Slug: d.Slug, State: d.State, Formats: d.Formats, Equivalent: true}
	bin, err := buildUmbra(demosRoot)
	if err != nil {
		return nil, workspace{}, err
	}
	w, err := newWorkspace(demosRoot, d.Slug)
	if err != nil {
		return nil, w, err
	}
	var first []StepResult
	for _, f := range d.Formats {
		steps, err := w.runFormat(demosRoot, bin, d, f)
		if err != nil {
			return nil, w, err
		}
		if first == nil {
			first = steps
			continue
		}
		for i := range steps {
			if steps[i] != first[i] {
				res.Equivalent = false
				res.Failures = append(res.Failures, fmt.Sprintf("%s refuses %q differently from %s: exit %d %q, not exit %d %q",
					f, steps[i].Cmd, d.Formats[0], steps[i].Exit, steps[i].Output, first[i].Exit, first[i].Output))
			}
		}
	}
	res.Steps = first
	res.check(d)
	return res, w, writeResult(w, res)
}

// runFormat installs the demo's policy in one format and runs every step
// against it, in that format's own fixture.
func (w workspace) runFormat(demosRoot, bin string, d *Demo, format string) ([]StepResult, error) {
	if err := w.prepare(demosRoot, bin, d, format); err != nil {
		return nil, err
	}
	var out []StepResult
	for _, s := range d.Steps {
		o, code := w.run(format, s.Cmd)
		out = append(out, StepResult{Cmd: s.Cmd, Exit: code, Output: o})
	}
	return out, nil
}

// prepare locks and installs the policy in one format, then builds its fixture.
func (w workspace) prepare(demosRoot, bin string, d *Demo, format string) error {
	for _, k := range []string{"build/.umbra", "shims", "fixture", "home"} {
		if err := os.MkdirAll(w.dir(k, format), 0o755); err != nil {
			return err
		}
	}
	src, err := os.ReadFile(d.Guardfile(format))
	if err != nil {
		return err
	}
	// Only this format's file, or discovery would see three members for one tool.
	if err := os.WriteFile(filepath.Join(w.dir("build/.umbra", format), filepath.Base(d.Guardfile(format))), src, 0o644); err != nil {
		return err
	}
	for _, args := range [][]string{{"lock", "--umbra-replace", filepath.Dir(demosRoot)}, {"install", "--shim-dir", w.dir("shims", format)}} {
		cmd := exec.Command(bin, args...)
		cmd.Dir = w.dir("build", format)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s (%s): umbra %s: %w\n%s", d.Slug, format, args[0], err, out)
		}
	}
	if _, err := os.Stat(filepath.Join(w.dir("shims", format), d.Tool)); err != nil {
		return fmt.Errorf("%s (%s): umbra installed no %q shim; does the guardfile wrap `exec %s` with `replace`?", d.Slug, format, d.Tool, d.Tool)
	}
	if err := w.setup(d, format); err != nil {
		return err
	}
	return w.writeEnv(format)
}

// check holds the filmed format's transcript to the manifest and the frame.
func (res *Result) check(d *Demo) {
	// The prompt line of every step plus the trailing prompt.
	res.Lines = len(d.Steps) + 1
	for i, s := range d.Steps {
		got := res.Steps[i]
		if got.Exit != s.Exit {
			res.Failures = append(res.Failures, fmt.Sprintf("%q exited %d, manifest says %d", s.Cmd, got.Exit, s.Exit))
		}
		if !strings.Contains(got.Output, s.Shows) {
			res.Failures = append(res.Failures, fmt.Sprintf("%q output lacks %q", s.Cmd, s.Shows))
		}
		for _, l := range append(strings.Split(got.Output, "\n"), "$ "+s.Cmd) {
			res.Widest = max(res.Widest, utf8.RuneCountInString(l))
		}
		if got.Output != "" {
			res.Lines += strings.Count(got.Output, "\n") + 1
		}
	}
	if res.Widest > d.Cols {
		res.Failures = append(res.Failures, fmt.Sprintf("widest line is %d columns, frame is %d: it would wrap on camera", res.Widest, d.Cols))
	}
	if res.Lines > d.Rows {
		res.Failures = append(res.Failures, fmt.Sprintf("transcript is %d lines, frame is %d: the first steps would scroll off", res.Lines, d.Rows))
	}
	res.Pass = len(res.Failures) == 0
}

func (w workspace) setup(d *Demo, format string) error {
	fixture := w.dir("fixture", format)
	for _, a := range d.Setup {
		var err error
		switch a.Kind {
		case "git-init":
			// The real git, before the shim is on PATH: setup is not on camera.
			cmd := exec.Command("git", "init", "-q", "-b", "main", ".")
			cmd.Dir = fixture
			err = cmd.Run()
		case "git-commit":
			// Fixed identity and dates, so the commit id is the same on every run.
			env := append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null",
				"GIT_AUTHOR_NAME=demo", "GIT_AUTHOR_EMAIL=demo@example.invalid", "GIT_AUTHOR_DATE=2026-01-01T00:00:00Z",
				"GIT_COMMITTER_NAME=demo", "GIT_COMMITTER_EMAIL=demo@example.invalid", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z")
			for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", a.Body}} {
				cmd := exec.Command("git", args...)
				cmd.Dir, cmd.Env = fixture, env
				if err = cmd.Run(); err != nil {
					break
				}
			}
		case "file":
			p := filepath.Join(fixture, a.Path)
			if err = os.MkdirAll(filepath.Dir(p), 0o755); err == nil {
				err = os.WriteFile(p, []byte(a.Body), 0o644)
			}
		case "guardfile":
			var b []byte
			if b, err = os.ReadFile(d.Guardfile(format)); err == nil {
				err = os.WriteFile(filepath.Join(fixture, a.Path), b, 0o644)
			}
		}
		if err != nil {
			return fmt.Errorf("%s (%s): setup %s: %w", d.Slug, format, a.Kind, err)
		}
	}
	return nil
}

func (w workspace) run(format, cmdline string) (string, int) {
	cmd := exec.Command("bash", "--noprofile", "--norc", "-c", "source "+w.envFile(format)+"\n"+cmdline)
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
