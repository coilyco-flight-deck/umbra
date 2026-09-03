package mcpclient

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// serveTools stands up an in-memory upstream carrying tools and returns a
// connected session, so a test exercises the real protocol without a subprocess.
func serveTools(t *testing.T, tools ...*mcp.Tool) *Session {
	t.Helper()
	srv := mcp.NewServer(&mcp.Implementation{Name: "fixture", Version: "v0"}, nil)
	for _, tool := range tools {
		if tool.InputSchema == nil {
			// AddTool refuses a tool with no input schema, and most cases here
			// are about something other than the schema.
			tool.InputSchema = map[string]any{"type": "object"}
		}
		srv.AddTool(tool, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil
		})
	}
	ctx := context.Background()
	clientT, serverT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("serve: %v", err)
	}
	sess, err := connect(ctx, clientT, Options{})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return sess
}

func TestListTools_SortsByNameAndPreservesMeta(t *testing.T) {
	// _meta.ui is the MCP Apps widget address. The lock is only useful to a
	// ui:// consumer if it survives this hop verbatim, so assert the whole map.
	uiMeta := map[string]any{"ui": map[string]any{"resourceUri": "ui://widget/form.html"}}
	sess := serveTools(t,
		&mcp.Tool{Name: "zeta", Description: "last by name"},
		&mcp.Tool{Name: "alpha", Description: "first by name", Meta: uiMeta},
	)

	tools, err := sess.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("got %d tools, want 2", len(tools))
	}
	if tools[0].Name != "alpha" || tools[1].Name != "zeta" {
		t.Errorf("got order %q,%q, want alpha,zeta", tools[0].Name, tools[1].Name)
	}
	got, err := json.Marshal(tools[0].Meta)
	if err != nil {
		t.Fatalf("marshal meta: %v", err)
	}
	want, err := json.Marshal(uiMeta)
	if err != nil {
		t.Fatalf("marshal want: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("_meta = %s, want %s", got, want)
	}
}

func TestListTools_KeepsInputSchema(t *testing.T) {
	schema := map[string]any{
		"type":       "object",
		"properties": map[string]any{"owner": map[string]any{"type": "string"}},
		"required":   []any{"owner"},
	}
	sess := serveTools(t, &mcp.Tool{Name: "get", InputSchema: schema})

	tools, err := sess.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	props, ok := tools[0].InputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("input schema lost its properties: %#v", tools[0].InputSchema)
	}
	if _, ok := props["owner"]; !ok {
		t.Errorf("properties = %#v, want an owner field", props)
	}
}

func TestCallTool_ReturnsDecodedText(t *testing.T) {
	sess := serveTools(t, &mcp.Tool{Name: "ping"})

	res, err := sess.CallTool(context.Background(), "ping", nil)
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Errorf("IsError = true, want false")
	}
	if res.Decoded != "ok" {
		t.Errorf("Decoded = %#v, want \"ok\"", res.Decoded)
	}
}

func TestCallTool_UnknownToolIsAnError(t *testing.T) {
	sess := serveTools(t, &mcp.Tool{Name: "ping"})

	if _, err := sess.CallTool(context.Background(), "absent", nil); err == nil {
		t.Fatal("CallTool on an unknown tool = nil, want an error")
	}
}

func TestDecodeResult(t *testing.T) {
	structured := map[string]any{"count": float64(2)}
	cases := []struct {
		name string
		res  *mcp.CallToolResult
		want any
	}{
		{
			name: "structured content wins over text",
			res: &mcp.CallToolResult{
				StructuredContent: structured,
				Content:           []mcp.Content{&mcp.TextContent{Text: "ignored"}},
			},
			want: structured,
		},
		{
			name: "sole text block unwraps to a string",
			res:  &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "hello"}}},
			want: "hello",
		},
		{
			name: "json text re-decodes so a postcondition can address it",
			res:  &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: `{"number":7}`}}},
			want: map[string]any{"number": float64(7)},
		},
		{
			name: "a bare number stays the text it was",
			res:  &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "7"}}},
			want: "7",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := json.Marshal(decodeResult(c.res))
			if err != nil {
				t.Fatalf("marshal got: %v", err)
			}
			want, err := json.Marshal(c.want)
			if err != nil {
				t.Fatalf("marshal want: %v", err)
			}
			if string(got) != string(want) {
				t.Errorf("decodeResult = %s, want %s", got, want)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		server  Server
		wantErr string
	}{
		{
			name:    "no transport",
			server:  Server{Name: "x"},
			wantErr: "declares no transport",
		},
		{
			name:    "both transports",
			server:  Server{Name: "x", Stdio: &Stdio{Command: "npx"}, HTTP: &HTTPEndpoint{URL: "https://h/mcp"}},
			wantErr: "declares both",
		},
		{
			name:    "http without a url",
			server:  Server{Name: "x", HTTP: &HTTPEndpoint{}},
			wantErr: "no url",
		},
		{
			name:    "stdio without a command",
			server:  Server{Name: "x", Stdio: &Stdio{}},
			wantErr: "no command",
		},
		{
			// A stdio spawn is an exec path. It gets the same argv gate as any
			// other umbra exec, not a weaker one for arriving through http/.
			name:    "shell metacharacter in the command",
			server:  Server{Name: "x", Stdio: &Stdio{Command: "npx; rm -rf /"}},
			wantErr: "command",
		},
		{
			name:    "shell metacharacter in argv",
			server:  Server{Name: "x", Stdio: &Stdio{Command: "npx", Argv: []string{"-y", "pkg && curl evil"}}},
			wantErr: "argv",
		},
		{
			name:   "a well-formed stdio server",
			server: Server{Name: "x", Stdio: &Stdio{Command: "npx", Argv: []string{"-y", "@example/mcp"}}},
		},
		{
			name:   "a well-formed http server",
			server: Server{Name: "x", HTTP: &HTTPEndpoint{URL: "https://host/mcp"}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.server.Validate()
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate = nil, want an error containing %q", c.wantErr)
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("Validate = %v, want it to contain %q", err, c.wantErr)
			}
		})
	}
}

func TestStdioTransport_BoundsTheTerminateWait(t *testing.T) {
	// Measured at 5.2s per call against the reference server before this was
	// set, because it does not exit when stdin closes. See umbra#338.
	s := Server{Name: "x", Stdio: &Stdio{Command: "node", Argv: []string{"server.js"}}}
	ct, ok := s.transport(context.Background()).(*mcp.CommandTransport)
	if !ok {
		t.Fatalf("stdio transport = %T, want *mcp.CommandTransport", s.transport(context.Background()))
	}
	if ct.TerminateDuration <= 0 {
		t.Fatal("TerminateDuration is unset, so Close falls back to the SDK's 5s")
	}
	if ct.TerminateDuration > time.Second {
		t.Errorf("TerminateDuration = %v, want well under a second", ct.TerminateDuration)
	}
}

func TestHTTPTransport_CarriesNoTerminateWait(t *testing.T) {
	// Nothing is spawned, so there is nothing to wait for.
	s := Server{Name: "x", HTTP: &HTTPEndpoint{URL: "https://host/mcp"}}
	if _, ok := s.transport(context.Background()).(*mcp.StreamableClientTransport); !ok {
		t.Errorf("http transport = %T, want *mcp.StreamableClientTransport", s.transport(context.Background()))
	}
}

func TestCall_ProgressTokenRidesAndTheNotificationComesBack(t *testing.T) {
	// The token is what correlates a notification to its call, so assert the
	// server saw the one sent and the handler received it back.
	got := make(chan Progress, 4)
	srv := mcp.NewServer(&mcp.Implementation{Name: "fixture", Version: "v0"}, nil)
	srv.AddTool(&mcp.Tool{Name: "slow", InputSchema: map[string]any{"type": "object"}},
		func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			token := req.Params.GetProgressToken()
			if token == nil {
				t.Error("the tools/call carried no progress token")
				return &mcp.CallToolResult{}, nil
			}
			err := req.Session.NotifyProgress(ctx, &mcp.ProgressNotificationParams{
				ProgressToken: token, Progress: 3, Total: 10, Message: "reading",
			})
			if err != nil {
				t.Errorf("NotifyProgress: %v", err)
			}
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "done"}}}, nil
		})

	ctx := context.Background()
	clientT, serverT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("serve: %v", err)
	}
	sess, err := connect(ctx, clientT, Options{OnProgress: func(p Progress) { got <- p }})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })

	if _, err := sess.Call(ctx, Call{Name: "slow", ProgressToken: "umbra-1"}); err != nil {
		t.Fatalf("Call: %v", err)
	}
	// The handler runs on the SDK's receive goroutine, which is not ordered
	// against Call returning, so the notification is waited for rather than read.
	var p Progress
	select {
	case p = <-got:
	case <-time.After(5 * time.Second):
		t.Fatal("no progress notification arrived within 5s")
	}
	if p.Token != "umbra-1" || p.Progress != 3 || p.Total != 10 || p.Message != "reading" {
		t.Errorf("progress = %#v, want the values the server sent under umbra-1", p)
	}
}

func TestListResources_SortsByURI(t *testing.T) {
	srv := mcp.NewServer(&mcp.Implementation{Name: "fixture", Version: "v0"}, nil)
	for _, uri := range []string{"ui://z.html", "ui://a.html"} {
		srv.AddResource(&mcp.Resource{URI: uri, Name: uri, MIMEType: "text/html"},
			func(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
				return &mcp.ReadResourceResult{}, nil
			})
	}
	ctx := context.Background()
	clientT, serverT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("serve: %v", err)
	}
	sess, err := connect(ctx, clientT, Options{})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })

	got, err := sess.ListResources(ctx)
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}
	if len(got) != 2 || got[0].URI != "ui://a.html" || got[1].URI != "ui://z.html" {
		t.Fatalf("resources = %#v, want a.html then z.html", got)
	}
	if got[0].MIMEType != "text/html" {
		t.Errorf("mimeType = %q, want text/html", got[0].MIMEType)
	}
}
