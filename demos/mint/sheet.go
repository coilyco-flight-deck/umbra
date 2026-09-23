package main

import (
	"encoding/json"
	"html/template"
	"os"
	"path/filepath"
)

// card is one demo on the review sheet: its manifest, its guardfile, and
// whatever the last verify or render left behind.
type card struct {
	Demo      *Demo
	Guardfile string
	Result    *Result
	Err       string
}

var sheetTmpl = template.Must(template.New("sheet").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>umbra demos</title>
<style>
:root { --bg:#111016; --fg:#e8e4f0; --dim:#9a93ab; --ok:#8fd694; --bad:#f28b82; --card:#1c1a24; --accent:#b48ead; }
body { background:var(--bg); color:var(--fg); font:15px/1.45 ui-monospace,Menlo,monospace; margin:0; padding:16px; }
main { display:grid; grid-template-columns:repeat(auto-fill,minmax(min(100%,560px),1fr)); gap:16px; }
section { background:var(--card); border-radius:10px; padding:14px; min-width:0; }
h2 { margin:0 0 6px; font-size:16px; color:var(--accent); }
.meta span { margin-right:12px; color:var(--dim); }
.pass { color:var(--ok); } .fail { color:var(--bad); }
video { width:100%; border-radius:6px; background:#000; }
pre { white-space:pre-wrap; overflow-wrap:anywhere; color:var(--dim); font-size:13px; }
ul { padding-left:18px; margin:6px 0; }
</style></head><body><main>
{{range .}}<section id="{{.Demo.Slug}}">
<h2>{{.Demo.Slug}}</h2>
<div class="meta"><span>tool {{.Demo.Tool}}</span><span>state {{.Demo.State}}</span>
{{with .Result}}<span class="{{if .Pass}}pass{{else}}fail{{end}}">{{if .Pass}}verified{{else}}failing{{end}}</span><span>{{.Widest}} cols / {{.Lines}} lines</span>{{if .Seconds}}<span>{{printf "%.1f" .Seconds}}s</span>{{end}}{{else}}<span class="fail">not run</span>{{end}}</div>
{{if .Err}}<pre class="fail">{{.Err}}</pre>{{end}}
{{with .Result}}{{if .Failures}}<ul class="fail">{{range .Failures}}<li>{{.}}</li>{{end}}</ul>{{end}}
{{if .Clip}}<video src="{{.Clip}}" controls muted loop playsinline></video>{{end}}
<ul>{{range .Steps}}<li>exit {{.Exit}} - {{.Cmd}}</li>{{end}}</ul>{{end}}
<pre>{{.Guardfile}}</pre>
</section>{{end}}
</main></body></html>
`))

func sheet(demosRoot string) (string, error) {
	slugs, err := minted(demosRoot)
	if err != nil {
		return "", err
	}
	var cards []card
	for _, s := range slugs {
		c := card{Demo: &Demo{Slug: s}}
		d, err := loadDemo(filepath.Join(demosRoot, s))
		if err != nil {
			c.Err = err.Error()
		} else {
			c.Demo = d
		}
		if g, _ := filepath.Glob(filepath.Join(demosRoot, s, ".umbra", "*.kdl")); len(g) > 0 {
			b, _ := os.ReadFile(g[0])
			c.Guardfile = string(b)
		}
		if b, err := os.ReadFile(filepath.Join(demosRoot, ".render", s, "result.json")); err == nil {
			var r Result
			if json.Unmarshal(b, &r) == nil {
				rel, _ := filepath.Rel(filepath.Join(demosRoot, ".render"), r.Clip)
				if r.Clip != "" {
					r.Clip = rel
				}
				c.Result = &r
			}
		}
		cards = append(cards, c)
	}
	out := filepath.Join(demosRoot, ".render", "index.html")
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return "", err
	}
	f, err := os.Create(out)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return out, sheetTmpl.Execute(f, cards)
}
