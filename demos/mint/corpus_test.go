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
		"duplicate":      {"call \"git log\" class=granted expect=accept\n    call \"git log\" class=granted expect=accept", "twice"},
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
