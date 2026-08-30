// Command mcpverb demonstrates the mcp dialect end-to-end: it starts its own
// MCP server, locks that server's tool surface, and mounts a guarded CLI over
// it. Self-contained on purpose, so the example needs no upstream to exist.
package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"

	"forgejo.coilysiren.me/coilyco-flight-deck/umbra/http/mcpverb"
	"forgejo.coilysiren.me/coilyco-flight-deck/umbra/pkg/mcpclient"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/urfave/cli/v3"
)

// policy grants one tool, denies another, and guards the granted one's owner.
// `search` is upstream and named by neither sentence, so it is absent too.
const policy = `wrap example ops demo {
    mcp http { url "%s" }
    can call list_issue {
        deny owner matches "^admin$"
    }
    never call delete_repository
}`

func main() {
	url := serve()
	gf, err := mcpverb.Parse([]byte(fmt.Sprintf(policy, url)))
	if err != nil {
		fail(err)
	}
	// Stands in for the committed lock `specgen lock` would write. A generated
	// binary reads its lock and never reaches the network to mount.
	tools, err := listTools(url)
	if err != nil {
		fail(err)
	}
	root := &cli.Command{Name: "example", Usage: "the mcp dialect, over a server this process started"}
	if err := mcpverb.Mount(root, mcpverb.Config{Guardfile: gf, Tools: tools}); err != nil {
		fail(err)
	}
	if err := root.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "refused:", err)
		os.Exit(2)
	}
}

// serve starts the upstream MCP server this example guards.
func serve() string {
	srv := mcp.NewServer(&mcp.Implementation{Name: "demo", Version: "v0"}, nil)
	srv.AddTool(&mcp.Tool{
		Name:        "list_issue",
		Description: "list issues in a repository",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"owner": map[string]any{"type": "string", "description": "repository owner"},
				"state": map[string]any{"type": "string", "enum": []any{"open", "closed"}},
			},
			"required": []any{"owner"},
		},
	}, func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf(`{"issues":2,"for":%s}`, req.Params.Arguments)},
		}}, nil
	})
	for _, name := range []string{"delete_repository", "search"} {
		srv.AddTool(&mcp.Tool{Name: name, InputSchema: map[string]any{"type": "object"}},
			func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil
			})
	}
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
	return httptest.NewServer(handler).URL
}

// listTools reads the upstream surface, the input `specgen lock` would freeze.
func listTools(url string) ([]mcpclient.Tool, error) {
	ctx := context.Background()
	sess, err := mcpclient.Connect(ctx, mcpclient.Server{Name: "demo", HTTP: &mcpclient.HTTPEndpoint{URL: url}})
	if err != nil {
		return nil, err
	}
	defer func() { _ = sess.Close() }()
	return sess.ListTools(ctx)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "example:", err)
	os.Exit(1)
}
