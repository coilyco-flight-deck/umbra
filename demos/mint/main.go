// Command mint turns a demos/<slug>/ manifest into a verified, recorded umbra
// demo. See demos/README.md.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const usage = `usage: mint <verb> [slug...]

  next             print the first backlog target that has no demo yet
  new <slug>       scaffold demos/<slug>/ from its backlog target
  verify <slug...> run every step in a clean fixture and check exit, output, and frame fit
  render <slug...> verify, then record the clip with the pinned vhs
  gif <slug...>    convert a rendered clip to gif, for markdown
  sheet            write .render/index.html, the review page
  status           list every target with its state

  --all in place of slugs means every minted demo.`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "mint:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New(usage)
	}
	root, err := demosRoot()
	if err != nil {
		return err
	}
	verb, rest := args[0], args[1:]
	switch verb {
	case "next":
		t, err := next(root)
		if err != nil {
			return err
		}
		fmt.Println(t.Slug)
		return nil
	case "new":
		if len(rest) != 1 {
			return errors.New("new takes one slug")
		}
		return scaffold(root, rest[0])
	case "verify", "render":
		return forEach(root, verb, rest)
	case "gif":
		for _, s := range rest {
			p, err := gif(root, s)
			if err != nil {
				return err
			}
			fmt.Println(p)
		}
		return nil
	case "sheet":
		p, err := sheet(root)
		if err == nil {
			fmt.Println(p)
		}
		return err
	case "status":
		return status(root)
	}
	return fmt.Errorf("unknown verb %q\n%s", verb, usage)
}

// demosRoot is the directory holding backlog.kdl, found from the working
// directory so the tool runs from demos/ or from the repository root alike.
func demosRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for _, c := range []string{wd, filepath.Join(wd, "demos")} {
		if _, err := os.Stat(filepath.Join(c, "backlog.kdl")); err == nil {
			return c, nil
		}
	}
	return "", errors.New("run from demos/ or the repository root: no backlog.kdl found")
}

func forEach(root, verb string, slugs []string) error {
	if len(slugs) == 1 && slugs[0] == "--all" {
		var err error
		if slugs, err = minted(root); err != nil {
			return err
		}
	}
	if len(slugs) == 0 {
		return fmt.Errorf("%s needs a slug or --all", verb)
	}
	var failed []string
	for _, s := range slugs {
		d, err := loadDemo(filepath.Join(root, s))
		if err == nil {
			var r *Result
			if verb == "render" {
				r, err = render(root, d)
			} else {
				r, _, err = verify(root, d)
			}
			if err == nil && !r.Pass {
				err = errors.New(strings.Join(r.Failures, "; "))
			}
			if err == nil {
				fmt.Printf("ok   %s\n", s)
				if r.Frame != "" {
					fmt.Printf("     look at %s before calling it good\n", r.Frame)
				}
				continue
			}
		}
		fmt.Printf("FAIL %s: %v\n", s, err)
		failed = append(failed, s)
	}
	if len(failed) > 0 {
		return fmt.Errorf("%d of %d failed: %s", len(failed), len(slugs), strings.Join(failed, ", "))
	}
	return nil
}

func next(root string) (Target, error) {
	targets, err := loadBacklog(root)
	if err != nil {
		return Target{}, err
	}
	done, err := minted(root)
	if err != nil {
		return Target{}, err
	}
	for _, t := range targets {
		if !contains(done, t.Slug) {
			return t, nil
		}
	}
	return Target{}, errors.New("backlog is empty: every target has a demo")
}

func scaffold(root, slug string) error {
	targets, err := loadBacklog(root)
	if err != nil {
		return err
	}
	var t *Target
	for i := range targets {
		if targets[i].Slug == slug {
			t = &targets[i]
		}
	}
	if t == nil {
		return fmt.Errorf("%q is not in backlog.kdl; add the target first", slug)
	}
	dir := filepath.Join(root, slug)
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("%s already exists", dir)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".umbra"), 0o755); err != nil {
		return err
	}
	manifest := fmt.Sprintf("demo %s {\n    tool %s\n    state minted\n    formats kdl yaml toml\n    frame cols=80 rows=16\n    setup {\n    }\n    step \"%s --help\" exit=0 shows=\"occluded by umbra\"\n}\n", slug, t.Tool, t.Tool)
	if err := os.WriteFile(filepath.Join(dir, "demo.kdl"), []byte(manifest), 0o644); err != nil {
		return err
	}
	// The exec grammar has no YAML or TOML schema yet, so exec and replace ride
	// the `kdl` escape inside wrap. See docs/guardfile-formats.md.
	guards := map[string]string{
		"kdl":  fmt.Sprintf("wrap demo %s {\n    exec %s\n    replace\n\n    can run version\n}\n", t.Tool, t.Tool),
		"yaml": fmt.Sprintf("wrap:\n  command: [demo, %s]\n  kdl: |\n    exec %s\n    replace\n  can:\n    - verb: run\n      resource: version\n", t.Tool, t.Tool),
		"toml": fmt.Sprintf("[wrap]\ncommand = [\"demo\", %q]\nkdl = \"\"\"\nexec %s\nreplace\n\"\"\"\n\n[[wrap.can]]\nverb = \"run\"\nresource = \"version\"\n", t.Tool, t.Tool),
	}
	for _, f := range allFormats {
		if err := os.WriteFile(filepath.Join(dir, ".umbra", t.Tool+".guardfile."+f), []byte(guards[f]), 0o644); err != nil {
			return err
		}
	}
	fmt.Println(dir)
	return nil
}

func status(root string) error {
	targets, err := loadBacklog(root)
	if err != nil {
		return err
	}
	for _, t := range targets {
		state := "queued"
		if d, err := loadDemo(filepath.Join(root, t.Slug)); err == nil {
			state = d.State
		} else if !errors.Is(err, os.ErrNotExist) {
			state = "broken: " + err.Error()
		}
		fmt.Printf("%-10s %-28s %s\n", state, t.Slug, t.Tool)
	}
	return nil
}
