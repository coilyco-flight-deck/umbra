package version_test

import (
	"os"
	"strings"
	"testing"
)

func mustRead(t *testing.T, path string) string {
	t.Helper()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	return string(b)
}

func TestReleasePipelinePinsDraftTagContract(t *testing.T) {
	promote := mustRead(t, "../../.forgejo/workflows/promote.yml")
	release := mustRead(t, "../../.forgejo/workflows/release.yml")
	publish := mustRead(t, "../../scripts/ci/publish-draft-tag.sh")
	doc := mustRead(t, "../../docs/release-pipeline.md")
	features := mustRead(t, "../../docs/FEATURES.md")

	checks := []struct {
		name string
		text string
		want string
	}{
		{
			// The derivation moved into the script promote.yml invokes, so the
			// contract is unchanged and the file that carries it is not.
			name: "promote draft tag",
			text: publish,
			want: "draft_tag=\"draft-${GITHUB_SHA}\"",
		},
		{
			name: "promote draft publish",
			text: promote,
			want: "Publish draft release tag",
		},
		{
			name: "release draft verify",
			text: release,
			want: "Verify the draft release tag was published on main",
		},
		{
			name: "release draft derivation",
			text: release,
			want: "draft_tag=draft-${GITHUB_SHA}",
		},
		{
			name: "docs draft publish",
			text: doc,
			want: "draft-${sha}",
		},
		{
			name: "features release inventory",
			text: features,
			want: "commit-scoped draft tags on `main`",
		},
	}

	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if !strings.Contains(check.text, check.want) {
				t.Fatalf("missing %q", check.want)
			}
		})
	}
}
