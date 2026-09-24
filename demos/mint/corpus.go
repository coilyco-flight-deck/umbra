package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	kdl "github.com/calico32/kdl-go"
)

// classes are the decision-boundary regions a corpus call is chosen to cover.
var classes = []string{"granted", "never", "withheld", "uncovered", "near-miss"}

// expects are what the generator asserts a call did: an audit row saying accept or
// reject, a refusal with no row, or umbra printing its own help with no row.
var expects = []string{"accept", "reject", "unaudited", "help"}

// buildSources are the paths the shim is compiled from. Their tree hash is the
// build id, so committing a regenerated corpus.jsonl never moves it.
var buildSources = []string{"cmd", "cli", "http", "internal", "pkg", "go.mod", "go.sum"}

// generatorSources are the paths that decide how a run becomes rows.
var generatorSources = []string{"demos/mint", "demos/go.mod", "demos/go.sum"}

// Corpus is one parsed corpora/<slug>/corpus.kdl.
type Corpus struct {
	Slug     string
	Dir      string
	Tool     string
	Requires string
	Setup    []SetupAction
	Calls    []Call
}

// Call is one invocation and the label it carries.
type Call struct {
	Cmd    string
	Class  string
	Expect string
	Rule   string
	Note   string
}

// Row is one corpus.jsonl line. Field order is the output order.
type Row struct {
	Corpus       string    `json:"corpus"`
	N            int       `json:"n"`
	Call         string    `json:"call"`
	Argv         []string  `json:"argv"`
	Class        string    `json:"class"`
	Expect       string    `json:"expect"`
	Rule         string    `json:"rule,omitempty"`
	Note         string    `json:"note,omitempty"`
	Build        string    `json:"build"`
	Generator    string    `json:"generator"`
	Requires     string    `json:"requires"`
	Guardfile    string    `json:"guardfile"`
	GuardfileSHA string    `json:"guardfile_sha256"`
	Exit         int       `json:"exit"`
	Output       string    `json:"output,omitempty"`
	Audit        *AuditRow `json:"audit"`
}

// AuditRow keeps the audit fields that are a function of the call alone. Ids,
// timestamps, durations and paths differ on every run and are dropped.
type AuditRow struct {
	Verb     string `json:"verb"`
	Decision string `json:"decision"`
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error,omitempty"`
}

func loadCorpus(dir string) (*Corpus, error) {
	src, err := os.ReadFile(filepath.Join(dir, "corpus.kdl"))
	if err != nil {
		return nil, err
	}
	doc, err := kdl.ParseString(string(src))
	if err != nil {
		return nil, fmt.Errorf("%s: parse KDL: %w", dir, err)
	}
	root := doc.GetNode("corpus")
	if root == nil || len(root.Arguments()) != 1 {
		return nil, fmt.Errorf("%s: needs exactly one top-level `corpus <slug>` node", dir)
	}
	c := &Corpus{Slug: root.Arg(0).String(), Dir: dir}
	if c.Slug != filepath.Base(dir) {
		return nil, fmt.Errorf("%s: corpus slug %q must match its directory", dir, c.Slug)
	}
	// Setup parsing is the demo manifest's, so both fixtures mean the same thing.
	scratch := &Demo{}
	for _, n := range root.Children().Nodes {
		switch n.Name() {
		case "tool":
			c.Tool = n.Arg(0).String()
		case "requires":
			c.Requires = n.Arg(0).String()
		case "setup":
			if err := scratch.apply(n); err != nil {
				return nil, fmt.Errorf("%s: %w", dir, err)
			}
		case "call":
			if len(n.Arguments()) != 1 || !n.Prop("class").IsValid() || !n.Prop("expect").IsValid() {
				return nil, fmt.Errorf("%s: `call` needs a command, class=<class> and expect=<expect>", dir)
			}
			call := Call{Cmd: n.Arg(0).String(), Class: n.Prop("class").String(), Expect: n.Prop("expect").String()}
			if v := n.Prop("note"); v.IsValid() {
				call.Note = v.String()
			}
			if v := n.Prop("rule"); v.IsValid() {
				call.Rule = v.String()
			}
			c.Calls = append(c.Calls, call)
		default:
			return nil, fmt.Errorf("%s: unknown node %q (tool, requires, setup, call)", dir, n.Name())
		}
	}
	c.Setup = scratch.Setup
	return c, c.validate()
}

func (c *Corpus) validate() error {
	if c.Tool == "" || c.Requires == "" {
		return fmt.Errorf("%s: needs `tool` and `requires <commit>`", c.Slug)
	}
	if _, err := os.Stat(c.demo().Guardfile("kdl")); err != nil {
		return fmt.Errorf("%s: no guardfile at .umbra/%s.guardfile.kdl", c.Slug, c.Tool)
	}
	seen := map[string]bool{}
	for _, call := range c.Calls {
		switch {
		case !contains(classes, call.Class):
			return fmt.Errorf("%s: %q class %q is not one of %s", c.Slug, call.Cmd, call.Class, strings.Join(classes, ", "))
		case !contains(expects, call.Expect):
			return fmt.Errorf("%s: %q expect %q is not one of %s", c.Slug, call.Cmd, call.Expect, strings.Join(expects, ", "))
		case strings.ContainsAny(call.Cmd, "\"'`$;&|<>\\"):
			// argv is the command split on spaces, so quoting would make it lie.
			return fmt.Errorf("%s: %q uses shell syntax; a call is plain space-separated words", c.Slug, call.Cmd)
		case seen[call.Cmd]:
			return fmt.Errorf("%s: %q appears twice", c.Slug, call.Cmd)
		case (call.Expect == "accept" || call.Expect == "reject") && call.Rule == "":
			// The label consumer drops one rule at a time and needs each row's own.
			return fmt.Errorf("%s: %q expects %s and names no rule=", c.Slug, call.Cmd, call.Expect)
		}
		if call.Rule != "" {
			if _, _, err := parseRule(call.Rule); err != nil {
				return fmt.Errorf("%s: %q: %w", c.Slug, call.Cmd, err)
			}
		}
		seen[call.Cmd] = true
	}
	if len(c.Calls) == 0 {
		return fmt.Errorf("%s: a corpus needs at least one call", c.Slug)
	}
	return nil
}

// demo presents the corpus to the demo workspace, which owns install and setup.
func (c *Corpus) demo() *Demo {
	return &Demo{Slug: "corpus-" + c.Slug, Dir: c.Dir, Tool: c.Tool, Formats: []string{"kdl"}, Setup: c.Setup}
}

// generate runs every call against a fresh install and returns the rows. It refuses
// an uncommitted build or one without the required fix: no row could name it.
func generate(demosRoot string, c *Corpus) ([]byte, error) {
	repo := filepath.Dir(demosRoot)
	build, err := treeID(repo, buildSources)
	if err != nil {
		return nil, err
	}
	gen, err := treeID(repo, generatorSources)
	if err != nil {
		return nil, err
	}
	if err := exec.Command("git", "-C", repo, "merge-base", "--is-ancestor", c.Requires, "HEAD").Run(); err != nil {
		return nil, fmt.Errorf("%s: HEAD does not contain required commit %s", c.Slug, c.Requires)
	}
	d := c.demo()
	gfPath := d.Guardfile("kdl")
	gf, err := os.ReadFile(gfPath)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(gf)

	bin, err := buildUmbra(demosRoot)
	if err != nil {
		return nil, err
	}
	w, err := newWorkspace(demosRoot, d.Slug)
	if err != nil {
		return nil, err
	}
	if err := w.prepare(demosRoot, bin, d, "kdl"); err != nil {
		return nil, err
	}
	auditDir := filepath.Join(w.dir("home", "kdl"), ".umbra", "audit")

	var out bytes.Buffer
	var failures []string
	for i, call := range c.Calls {
		before, err := auditLines(auditDir)
		if err != nil {
			return nil, err
		}
		output, code := w.run("kdl", call.Cmd)
		after, err := auditLines(auditDir)
		if err != nil {
			return nil, err
		}
		fresh := after[len(before):]
		if len(fresh) > 1 {
			return nil, fmt.Errorf("%s: %q wrote %d audit rows, a corpus row holds one", c.Slug, call.Cmd, len(fresh))
		}
		row := Row{
			Corpus: c.Slug, N: i + 1, Call: call.Cmd, Argv: strings.Fields(call.Cmd),
			Class: call.Class, Expect: call.Expect, Rule: call.Rule, Note: call.Note,
			Build: build, Generator: gen, Requires: c.Requires,
			Guardfile: filepath.Base(gfPath), GuardfileSHA: hex.EncodeToString(sum[:]),
			Exit: code,
		}
		if len(fresh) == 1 {
			if row.Audit, err = parseAudit(fresh[0]); err != nil {
				return nil, fmt.Errorf("%s: %q: %w", c.Slug, call.Cmd, err)
			}
		}
		// The wrapped tool's own output varies by its version, so only umbra's
		// refusal text is kept.
		if call.Expect != "accept" {
			row.Output = scrub(output, w.root)
		}
		if got := observed(row); got != call.Expect {
			failures = append(failures, fmt.Sprintf("%q is labelled expect=%s but was %s (exit %d)", call.Cmd, call.Expect, got, code))
		} else if err := ruleDecided(row); err != nil {
			failures = append(failures, fmt.Sprintf("%q: %v", call.Cmd, err))
		}
		b, err := json.Marshal(row)
		if err != nil {
			return nil, err
		}
		out.Write(append(b, '\n'))
	}
	if len(failures) > 0 {
		return nil, errors.New(strings.Join(failures, "\n"))
	}
	return out.Bytes(), nil
}

// observed names what a call actually did, in the vocabulary of expects.
func observed(r Row) string {
	switch {
	case r.Audit == nil && r.Exit != 0:
		return "unaudited"
	case r.Audit == nil:
		return "help"
	default:
		return r.Audit.Decision
	}
}

// parseRule splits a declared rule into its kind and the word it names, e.g.
// `never run reflog expire` into ("never run", "reflog expire").
func parseRule(rule string) (kind, arg string, err error) {
	if rule == "uncovered" {
		return rule, "", nil
	}
	for _, k := range []string{"can run", "never run", "withhold", "deny-flag", "deny-when"} {
		if rest, ok := strings.CutPrefix(rule, k+" "); ok && rest != "" {
			return k, rest, nil
		}
	}
	return "", "", fmt.Errorf("rule %q is not `uncovered` or one of can run, never run, withhold, deny-flag, deny-when", rule)
}

// ruleDecided checks the declared rule against what the call observably did, so
// a row cannot carry a rule some other line of the guardfile decided.
func ruleDecided(r Row) error {
	kind, arg, _ := parseRule(r.Rule)
	var want string
	switch kind {
	case "":
		return nil
	case "can run":
		if r.Audit != nil && strings.HasSuffix(r.Audit.Verb, "."+strings.ReplaceAll(arg, " ", ".")) {
			return nil
		}
		return fmt.Errorf("rule %q, but the audit row names %v", r.Rule, r.Audit)
	case "uncovered":
		// A root flag is refused before any verb resolves, with its own text.
		if strings.Contains(r.Output, "flag provided but not defined") {
			return nil
		}
		want = "is not granted"
	case "never run":
		want = "`" + arg + "` is never allowed"
	case "withhold":
		want = "`" + arg + "` is withheld"
	case "deny-flag":
		want = fmt.Sprintf("flag %q is denied", arg)
	case "deny-when":
		want = fmt.Sprintf("matched %q", arg)
	}
	if !strings.Contains(r.Output, want) {
		return fmt.Errorf("rule %q, but the refusal does not say %q", r.Rule, want)
	}
	return nil
}

// treeID hashes the committed trees of paths at HEAD, refusing uncommitted
// changes to them, since a dirty tree is a build nothing can name.
func treeID(repo string, paths []string) (string, error) {
	status, err := exec.Command("git", append([]string{"-C", repo, "status", "--porcelain", "--"}, paths...)...).Output()
	if err != nil {
		return "", err
	}
	if len(bytes.TrimSpace(status)) > 0 {
		return "", fmt.Errorf("commit these first, a corpus names its build by commit:\n%s", status)
	}
	tree, err := exec.Command("git", append([]string{"-C", repo, "ls-tree", "HEAD", "--"}, paths...)...).Output()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(tree)
	return hex.EncodeToString(sum[:]), nil
}

func auditLines(dir string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	var lines []string
	for _, f := range files {
		fh, err := os.Open(f)
		if err != nil {
			return nil, err
		}
		s := bufio.NewScanner(fh)
		s.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for s.Scan() {
			if t := strings.TrimSpace(s.Text()); t != "" {
				lines = append(lines, t)
			}
		}
		fh.Close()
		if err := s.Err(); err != nil {
			return nil, err
		}
	}
	return lines, nil
}

func parseAudit(line string) (*AuditRow, error) {
	var a AuditRow
	if err := json.Unmarshal([]byte(line), &a); err != nil {
		return nil, fmt.Errorf("audit row: %w", err)
	}
	return &a, nil
}

// scrub replaces the run's workspace path, so the output is the same on any host.
func scrub(s, root string) string {
	return strings.ReplaceAll(s, root, "<workspace>")
}

// corpus regenerates each named corpus into its corpus.jsonl, or with check
// compares the regeneration to the committed file and writes nothing.
func corpus(demosRoot string, args []string) error {
	check := len(args) > 0 && args[0] == "--check"
	if check {
		args = args[1:]
	}
	if len(args) == 1 && args[0] == "--all" {
		m, err := filepath.Glob(filepath.Join(demosRoot, "corpora", "*", "corpus.kdl"))
		if err != nil {
			return err
		}
		args = nil
		for _, p := range m {
			args = append(args, filepath.Base(filepath.Dir(p)))
		}
	}
	if len(args) == 0 {
		return errors.New("corpus needs a slug or --all")
	}
	for _, slug := range args {
		c, err := loadCorpus(filepath.Join(demosRoot, "corpora", slug))
		if err != nil {
			return err
		}
		rows, err := generate(demosRoot, c)
		if err != nil {
			return err
		}
		dst := filepath.Join(c.Dir, "corpus.jsonl")
		if check {
			have, err := os.ReadFile(dst)
			if err != nil {
				return err
			}
			if !bytes.Equal(have, rows) {
				return fmt.Errorf("%s: corpus.jsonl differs from a fresh regeneration; run `just corpus %s`", slug, slug)
			}
			fmt.Printf("ok   %s matches\n", slug)
			continue
		}
		if err := os.WriteFile(dst, rows, 0o644); err != nil {
			return err
		}
		fmt.Printf("ok   %s: %d rows\n", slug, len(c.Calls))
	}
	return nil
}
