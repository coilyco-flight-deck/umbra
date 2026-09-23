package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const fontSize = 22

// Geometry measured off a rendered frame, 0.68em a column and 1.23em a line.
// Rounded to even for libx264, see docs/demos.md.
func frameSize(d *Demo) (int, int) {
	even := func(f float64) int { return (int(f) + 1) &^ 1 }
	return even(float64(d.Cols)*0.68*fontSize + 120), even(float64(d.Rows)*1.25*fontSize + 120)
}

func tape(d *Demo, w workspace) string {
	width, height := frameSize(d)
	var b strings.Builder
	fmt.Fprintf(&b, "Output %q\n", filepath.Join(w.root, "clip.mp4"))
	fmt.Fprintf(&b, "Set Shell \"bash\"\nSet FontSize %d\nSet Width %d\nSet Height %d\n", fontSize, width, height)
	b.WriteString("Set TypingSpeed 45ms\nSet Framerate 30\nSet Theme \"Catppuccin Mocha\"\nSet Padding 30\n")
	// Off-camera lines type at 1ms: at the on-camera speed the env line alone
	// cost 23 seconds a render.
	fmt.Fprintf(&b, "Hide\nType@1ms %q\nEnter\nType@1ms \"clear\"\nEnter\nWait+Line /\\$\\s*$/\nShow\nSleep 500ms\n", "source "+w.envFile(d.Formats[0]))
	for _, s := range d.Steps {
		// Wait on the prompt rather than sleeping: a sleep too short types the
		// next command into the previous one's output.
		fmt.Fprintf(&b, "Type %s\nSleep 400ms\nEnter\nWait+Line@20s /\\$\\s*$/\nSleep 1800ms\n", quoteTape(s.Cmd))
	}
	b.WriteString("Sleep 2500ms\n")
	return b.String()
}

// quoteTape picks the vhs string delimiter the command does not contain.
func quoteTape(s string) string {
	if strings.Contains(s, `"`) {
		return "`" + s + "`"
	}
	return `"` + s + `"`
}

func render(demosRoot string, d *Demo) (*Result, error) {
	res, w, err := verify(demosRoot, d)
	if err != nil {
		return nil, err
	}
	if !res.Pass {
		return res, fmt.Errorf("%s: verify failed, not rendering:\n  %s", d.Slug, strings.Join(res.Failures, "\n  "))
	}
	tp := filepath.Join(w.root, "demo.tape")
	if err := os.WriteFile(tp, []byte(tape(d, w)), 0o644); err != nil {
		return nil, err
	}
	// vhs is pinned in go.mod as a tool: v0.12.0 exits 0 and writes nothing
	// (charmbracelet/vhs#787), so the version is part of the harness.
	cmd := exec.Command("go", "tool", "vhs", tp)
	cmd.Dir = demosRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("%s: vhs: %w\n%s", d.Slug, err, out)
	}
	res.Clip = filepath.Join(w.root, "clip.mp4")
	// vhs's exit code does not prove a file was written, so probe the file.
	probe, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", res.Clip).Output()
	if err != nil {
		return nil, fmt.Errorf("%s: vhs wrote no readable clip: %w", d.Slug, err)
	}
	res.Seconds, _ = strconv.ParseFloat(strings.TrimSpace(string(probe)), 64)
	res.Frame = filepath.Join(w.root, "last.png")
	if out, err := exec.Command("ffmpeg", "-v", "error", "-sseof", "-0.3", "-i", res.Clip, "-vframes", "1", res.Frame, "-y").CombinedOutput(); err != nil {
		return nil, fmt.Errorf("%s: extract last frame: %w\n%s", d.Slug, err, out)
	}
	return res, writeResult(w, res)
}

// gif converts an already rendered clip for markdown, which cannot embed mp4.
// It stays out of render because encoding both doubled every render's cost.
func gif(demosRoot, slug string) (string, error) {
	dir := filepath.Join(demosRoot, ".render", slug)
	in, out := filepath.Join(dir, "clip.mp4"), filepath.Join(dir, "clip.gif")
	if _, err := os.Stat(in); err != nil {
		return "", fmt.Errorf("%s: render it first: %w", slug, err)
	}
	vf := "fps=15,split[a][b];[a]palettegen=max_colors=64[p];[b][p]paletteuse=dither=none"
	if o, err := exec.Command("ffmpeg", "-v", "error", "-i", in, "-vf", vf, out, "-y").CombinedOutput(); err != nil {
		return "", fmt.Errorf("%s: gif: %w\n%s", slug, err, o)
	}
	return out, nil
}
