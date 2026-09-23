package main

import (
	"encoding/json"
	"html/template"
	"os"
	"path/filepath"
	"strings"
)

// card is one demo on the review sheet: its manifest, its guardfile in every
// format, and whatever the last verify or render left behind.
type card struct {
	Demo       *Demo
	Guardfiles []source
	Result     *Result
	Err        string
}

type source struct{ Format, Text string }

var sheetTmpl = template.Must(template.New("sheet").Funcs(template.FuncMap{"join": strings.Join}).Parse(`<!doctype html>
<html lang="en" data-fmt="kdl"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>umbra demos</title>
<style>
:root { --bg:#111016; --fg:#e8e4f0; --dim:#9a93ab; --ok:#8fd694; --bad:#f28b82; --card:#1c1a24; --accent:#b48ead; }
body { background:var(--bg); color:var(--fg); font:15px/1.45 ui-monospace,Menlo,monospace; margin:0; padding:16px; }
header { display:flex; gap:8px; align-items:center; margin-bottom:16px; flex-wrap:wrap; }
header button { font:inherit; background:var(--card); color:var(--dim); border:1px solid #333; border-radius:6px; padding:6px 14px; cursor:pointer; }
html[data-fmt=kdl] button[data-fmt=kdl], html[data-fmt=yaml] button[data-fmt=yaml], html[data-fmt=toml] button[data-fmt=toml] { color:var(--fg); border-color:var(--accent); }
main { display:grid; grid-template-columns:repeat(auto-fill,minmax(min(100%,560px),1fr)); gap:16px; }
section { background:var(--card); border-radius:10px; padding:14px; min-width:0; }
h2 { margin:0 0 6px; font-size:16px; color:var(--accent); }
.meta span { margin-right:12px; color:var(--dim); }
.pass { color:var(--ok); } .fail { color:var(--bad); }
video { width:100%; border-radius:6px; background:#000; }
pre { white-space:pre-wrap; overflow-wrap:anywhere; color:var(--dim); font-size:13px; display:none; }
html[data-fmt=kdl] pre.src-kdl, html[data-fmt=yaml] pre.src-yaml, html[data-fmt=toml] pre.src-toml, pre.err { display:block; }
ul { padding-left:18px; margin:6px 0; }
</style></head><body>
<header><span>guardfile</span><button data-fmt="kdl">kdl</button><button data-fmt="yaml">yaml</button><button data-fmt="toml">toml</button></header>
<main>
{{range .}}<section id="{{.Demo.Slug}}">
<h2>{{.Demo.Slug}}</h2>
<div class="meta"><span>tool {{.Demo.Tool}}</span><span>state {{.Demo.State}}</span>
{{with .Result}}<span class="{{if .Pass}}pass{{else}}fail{{end}}">{{if .Pass}}verified{{else}}failing{{end}}</span><span class="{{if .Equivalent}}pass{{else}}fail{{end}}">{{if .Equivalent}}identical{{else}}diverges{{end}} across {{join .Formats " / "}}</span><span>{{.Widest}} cols / {{.Lines}} lines</span>{{if .Seconds}}<span>{{printf "%.1f" .Seconds}}s</span>{{end}}{{else}}<span class="fail">not run</span>{{end}}</div>
{{if .Err}}<pre class="err fail">{{.Err}}</pre>{{end}}
{{with .Result}}{{if .Failures}}<ul class="fail">{{range .Failures}}<li>{{.}}</li>{{end}}</ul>{{end}}
{{if .Clip}}<video src="{{.Clip}}" controls muted loop playsinline></video>{{end}}
<ul>{{range .Steps}}<li>exit {{.Exit}} - {{.Cmd}}</li>{{end}}</ul>{{end}}
{{range .Guardfiles}}<pre class="src-{{.Format}}">{{.Text}}</pre>{{end}}
</section>{{end}}
</main>
<script>
const root = document.documentElement;
const pick = f => { root.dataset.fmt = f; try { localStorage.setItem("fmt", f); } catch (e) {} };
try { const s = localStorage.getItem("fmt"); if (s) root.dataset.fmt = s; } catch (e) {}
document.querySelectorAll("header button").forEach(b => b.onclick = () => pick(b.dataset.fmt));
document.addEventListener("keydown", e => {
  if (e.key !== "f") return;
  const order = ["kdl", "yaml", "toml"];
  pick(order[(order.indexOf(root.dataset.fmt) + 1) % order.length]);
});
</script>
</body></html>
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
			for _, f := range d.Formats {
				b, _ := os.ReadFile(d.Guardfile(f))
				c.Guardfiles = append(c.Guardfiles, source{f, string(b)})
			}
		}
		if b, err := os.ReadFile(filepath.Join(demosRoot, ".render", s, "result.json")); err == nil {
			var r Result
			if json.Unmarshal(b, &r) == nil {
				if r.Clip != "" {
					r.Clip, _ = filepath.Rel(filepath.Join(demosRoot, ".render"), r.Clip)
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
