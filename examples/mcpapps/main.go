// Command mcpapps demonstrates the MCP Apps host bridge under a guardfile: it
// starts its own MCP server carrying a tool with a `ui://` view, resolves that
// view's `widget` block into a policy, and replays the frame sequence a real
// widget sends. Self-contained on purpose, so the example needs no browser and
// no upstream to exist.
//
// The frames below are the ones measured against a live widget, in the order it
// sends them. The eleventh is the one the spike this replaces forwarded without
// asking anything: here it is refused, and the view is told so.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"

	"forgejo.coilysiren.me/coilyco-flight-deck/umbra/http/mcpverb"
	"forgejo.coilysiren.me/coilyco-flight-deck/umbra/pkg/mcpapps"
	"forgejo.coilysiren.me/coilyco-flight-deck/umbra/pkg/mcpclient"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// hostTool is the tool whose result instantiates the view.
const hostTool = "get_system_info"

// policy grants the view one call and one read. `wipe_disk` is a CLI leaf here
// and the view still cannot reach it: a widget block is its own surface.
const policy = `wrap example ops monitor {
    mcp http { url "%s" }
    can call get_system_info {
        widget {
            can call poll_system_stats {
                deny scope matches "^secret"
            }
            can read "^ui://"
            can open "^https://"
            can save "^ui://"
        }
    }
    can call wipe_disk { destructive }
}`

// reported is closed once a progress frame has reached the view, because the SDK
// dispatches one on its own goroutine. The upstream tool waits on it.
var (
	reported = make(chan struct{})
	once     sync.Once
)

func main() {
	ctx := context.Background()
	url := serve()

	gf, err := mcpverb.Parse([]byte(fmt.Sprintf(policy, url)))
	if err != nil {
		fail(err)
	}
	// Stands in for the committed lock `specgen lock` would write.
	// Built first, so its progress sink is wired into the session below.
	host := &mcpapps.Host{
		Info:    mcpapps.Implementation{Name: "umbra-example-host", Version: "v0"},
		Context: mcpapps.HostContext{"theme": "dark", "displayMode": "inline"},
	}
	host.Emit = func(r mcpapps.Reply) {
		printFrame("->", r.Method(), r)
		once.Do(func() { close(reported) })
	}
	host.OpenLink = func(_ context.Context, target string) error {
		fmt.Printf("   host would open %s\n", target)
		return nil
	}
	host.SaveFile = func(_ context.Context, items []mcpapps.DownloadItem) error {
		for _, it := range items {
			fmt.Printf("   host would save %s (%s)\n", it.URI, it.MIMEType)
		}
		return nil
	}
	sess, err := mcpclient.ConnectWith(ctx,
		mcpclient.Server{Name: "monitor", HTTP: &mcpclient.HTTPEndpoint{URL: url}},
		mcpclient.Options{OnProgress: host.HandleProgress})
	if err != nil {
		fail(err)
	}
	defer func() { _ = sess.Close() }()
	tools, err := sess.ListTools(ctx)
	if err != nil {
		fail(err)
	}

	cfg := mcpverb.Config{Guardfile: gf, Tools: tools}
	pol, err := mcpverb.WidgetPolicy(cfg, hostTool)
	if err != nil {
		fail(err)
	}
	instantiating, err := sess.CallTool(ctx, hostTool, map[string]any{})
	if err != nil {
		fail(err)
	}
	host.Instantiating = map[string]any{"content": []any{map[string]any{"type": "text", "text": string(instantiating.Raw)}}}
	host.Session = sess
	host.Policy = pol

	for _, t := range tools {
		if uri := mcpapps.WidgetURI(t); uri != "" {
			fmt.Printf("view for %s lives at %s\n\n", t.Name, uri)
		}
	}
	for _, f := range script() {
		replay(ctx, host, f)
	}
}

// frame is one inbound view frame plus the label the log prints for it.
type frame struct {
	label string
	body  map[string]any
}

// script is the frame sequence a real widget sends, ending on the two calls
// that separate a granted view call from one the guardfile refuses.
func script() []frame {
	return []frame{
		{"ui/initialize", map[string]any{"jsonrpc": "2.0", "id": 0, "method": mcpapps.MethodInitialize,
			"params": map[string]any{"appInfo": map[string]any{"name": "System Monitor", "version": "1.0.0"}}}},
		{"ui/notifications/initialized", map[string]any{"jsonrpc": "2.0", "method": mcpapps.MethodInitialized}},
		{"ui/notifications/size-changed", map[string]any{"jsonrpc": "2.0", "method": mcpapps.MethodSizeChanged,
			"params": map[string]any{"width": 737, "height": 558}}},
		{"tools/list", map[string]any{"jsonrpc": "2.0", "id": 1, "method": mcpapps.MethodToolsList}},
		{"tools/call poll_system_stats", map[string]any{"jsonrpc": "2.0", "id": 2, "method": mcpapps.MethodToolsCall,
			"params": map[string]any{"name": "poll_system_stats", "arguments": map[string]any{"scope": "cpu"}}}},
		{"tools/call poll_system_stats (with a progress token)", map[string]any{"jsonrpc": "2.0", "id": 7, "method": mcpapps.MethodToolsCall,
			"params": map[string]any{"name": "poll_system_stats", "arguments": map[string]any{"scope": "cpu"},
				"_meta": map[string]any{"progressToken": "view-1"}}}},
		{"tools/call poll_system_stats (guarded argument)", map[string]any{"jsonrpc": "2.0", "id": 3, "method": mcpapps.MethodToolsCall,
			"params": map[string]any{"name": "poll_system_stats", "arguments": map[string]any{"scope": "secret-partition"}}}},
		{"tools/call wipe_disk (ungranted to the view)", map[string]any{"jsonrpc": "2.0", "id": 4, "method": mcpapps.MethodToolsCall,
			"params": map[string]any{"name": "wipe_disk", "arguments": map[string]any{}}}},
		{"resources/read file:///etc/passwd", map[string]any{"jsonrpc": "2.0", "id": 5, "method": mcpapps.MethodResourcesRead,
			"params": map[string]any{"uri": "file:///etc/passwd"}}},
		{"resources/list", map[string]any{"jsonrpc": "2.0", "id": 6, "method": mcpapps.MethodResourcesList}},
		{"ui/open-link https://example.com", map[string]any{"jsonrpc": "2.0", "id": 8, "method": mcpapps.MethodOpenLink,
			"params": map[string]any{"url": "https://example.com"}}},
		{"ui/open-link http://example.com (not https)", map[string]any{"jsonrpc": "2.0", "id": 9, "method": mcpapps.MethodOpenLink,
			"params": map[string]any{"url": "http://example.com"}}},
		{"ui/download-file ui://report.csv", map[string]any{"jsonrpc": "2.0", "id": 10, "method": mcpapps.MethodDownloadFile,
			"params": map[string]any{"contents": []any{map[string]any{
				"type": "resource", "resource": map[string]any{"uri": "ui://report.csv", "mimeType": "text/csv"}}}}}},
		{"ui/download-file file:///etc/passwd", map[string]any{"jsonrpc": "2.0", "id": 11, "method": mcpapps.MethodDownloadFile,
			"params": map[string]any{"contents": []any{map[string]any{
				"type": "resource_link", "uri": "file:///etc/passwd"}}}}},
		{"ui/request-display-mode (undeclared capability)", map[string]any{"jsonrpc": "2.0", "id": 12, "method": "ui/request-display-mode",
			"params": map[string]any{"mode": "fullscreen"}}},
	}
}

// replay hands one frame to the host and prints what came back, in the shape a
// frame log reads: the view's frame, then every reply it produced.
func replay(ctx context.Context, host *mcpapps.Host, f frame) {
	raw, err := json.Marshal(f.body)
	if err != nil {
		fail(err)
	}
	fmt.Printf("<- %s\n", f.label)
	replies := host.Handle(ctx, raw)
	if len(replies) == 0 {
		fmt.Println("   (no reply: a notification needs none)")
		return
	}
	for _, r := range replies {
		printFrame("->", r.Method(), r)
	}
}

// printFrame renders one outbound frame, for a reply and for an unsolicited
// notification alike.
func printFrame(dir, method string, r mcpapps.Reply) {
	out, err := json.Marshal(r)
	if err != nil {
		fail(err)
	}
	kind := "result"
	switch {
	case method != "":
		kind = method
	case r.IsError():
		kind = "refused"
	}
	fmt.Printf("%s %-10s %s\n", dir, kind, truncate(string(out)))
}

// truncate keeps the log readable when a result is large.
func truncate(s string) string {
	if len(s) <= 220 {
		return s
	}
	return s[:220] + "..."
}

// serve starts the upstream this example guards: one tool carrying a view, one
// the view polls, and one it must not reach.
func serve() string {
	srv := mcp.NewServer(&mcp.Implementation{Name: "monitor", Version: "v0"}, nil)
	srv.AddTool(&mcp.Tool{
		Name:        hostTool,
		Description: "system information, rendered as an MCP App",
		InputSchema: map[string]any{"type": "object"},
		// _meta.ui is the view's address, preserved verbatim through the lock.
		Meta: map[string]any{"ui": map[string]any{"resourceUri": "ui://monitor/view.html"}},
	}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{StructuredContent: map[string]any{"host": "example.local", "platform": "darwin arm64"}}, nil
	})
	srv.AddTool(&mcp.Tool{
		Name:        "poll_system_stats",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{"scope": map[string]any{"type": "string"}}},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			Scope string `json:"scope"`
		}
		_ = json.Unmarshal(req.Params.Arguments, &args)
		if token := req.Params.GetProgressToken(); token != nil {
			_ = req.Session.NotifyProgress(ctx, &mcp.ProgressNotificationParams{
				ProgressToken: token, Progress: 1, Total: 2, Message: "sampling",
			})
			select {
			case <-reported:
			case <-ctx.Done():
			}
		}
		return &mcp.CallToolResult{StructuredContent: map[string]any{
			"uptime": "13h 38m", "memory": "5.4 GB / 18.0 GB", "scope": args.Scope,
		}}, nil
	})
	for _, uri := range []string{"ui://monitor/view.html", "file:///etc/passwd"} {
		srv.AddResource(&mcp.Resource{URI: uri, Name: uri, MIMEType: "text/plain"},
			func(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
				return &mcp.ReadResourceResult{}, nil
			})
	}
	srv.AddTool(&mcp.Tool{Name: "wipe_disk", InputSchema: map[string]any{"type": "object"}},
		func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Never reached: the view's grant does not name it, so the refusal
			// happens in the host rather than here.
			return &mcp.CallToolResult{StructuredContent: map[string]any{"wiped": true}}, nil
		})
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
	return httptest.NewServer(handler).URL
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "example:", err)
	os.Exit(1)
}
