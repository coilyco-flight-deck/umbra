package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeCorpus(t *testing.T, body string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "sample")
	if err := os.MkdirAll(filepath.Join(dir, ".umbra"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".umbra", "git.guardfile.kdl"), []byte("wrap t git {\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "corpus.kdl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoadCorpusRejectsBadCalls(t *testing.T) {
	for name, tc := range map[string]struct{ call, want string }{
		"shell syntax":   {`call "git log | head" class=granted expect=accept`, "shell syntax"},
		"unknown class":  {`call "git log" class=maybe expect=accept`, "class"},
		"unknown expect": {`call "git log" class=granted expect=allow`, "expect"},
		"duplicate":      {"call \"git log\" class=granted expect=accept rule=\"can run log\"\n    call \"git log\" class=granted expect=accept rule=\"can run log\"", "twice"},
		"no rule":        {`call "git log" class=granted expect=accept`, "names no rule"},
		"unknown rule":   {`call "git log" class=granted expect=accept rule="allow log"`, "is not `uncovered`"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := writeCorpus(t, "corpus sample {\n    tool git\n    requires \"abc\"\n    "+tc.call+"\n}\n")
			_, err := loadCorpus(dir)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestObservedNamesEveryOutcome(t *testing.T) {
	for want, r := range map[string]Row{
		"accept":    {Exit: 0, Audit: &AuditRow{Decision: "accept"}},
		"reject":    {Exit: 2, Audit: &AuditRow{Decision: "reject"}},
		"unaudited": {Exit: 2},
		"help":      {Exit: 0},
	} {
		if got := observed(r); got != want {
			t.Errorf("observed(%+v) = %q, want %q", r, got, want)
		}
	}
}

func TestScrubRemovesWorkspace(t *testing.T) {
	if got := scrub("/tmp/ws/kdl/fixture: denied", "/tmp/ws"); got != "<workspace>/kdl/fixture: denied" {
		t.Errorf("scrub = %q", got)
	}
}

// A declared rule has to be the one the observed refusal or audit row names.
func TestRuleDecidedChecksTheObservedOutcome(t *testing.T) {
	for name, tc := range map[string]struct {
		row Row
		ok  bool
	}{
		"never holds":       {Row{Rule: "never run gc", Output: "git: `gc` is never allowed by this guardfile"}, true},
		"never mislabelled": {Row{Rule: "never run gc", Output: "git: `git gc` is not granted"}, false},
		"withhold holds":    {Row{Rule: "withhold push", Output: "git: `push` is withheld: no remote"}, true},
		"deny-flag holds":   {Row{Rule: "deny-flag --amend", Output: `git: flag "--amend" is denied for ` + "`commit`"}, true},
		"deny-when holds":   {Row{Rule: "deny-when *secret*", Output: `git: ` + "`log`" + ` denied: any-arg "s" matched "*secret*"`}, true},
		"uncovered holds":   {Row{Rule: "uncovered", Output: "git: `git fetch` is not granted"}, true},
		"can run holds":     {Row{Rule: "can run stash list", Audit: &AuditRow{Verb: "corpus.git.stash.list"}}, true},
		"can run elsewhere": {Row{Rule: "can run stash list", Audit: &AuditRow{Verb: "corpus.git.status"}}, false},
	} {
		if err := ruleDecided(tc.row); (err == nil) != tc.ok {
			t.Errorf("%s: ruleDecided = %v, want ok=%v", name, err, tc.ok)
		}
	}
}
