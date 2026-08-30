// Package mcpclient is the Model Context Protocol client umbra's mcp dialect
// speaks: one declared upstream server, a session over it, and the three calls
// the dialect needs (tools/list, tools/call, resources/read).
//
// It lives in pkg/ rather than a guarded surface because it expresses no
// permission. Policy is the guardfile's, enforced above this by http/mcpverb.
// See docs/mcpverb.md.
package mcpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"sort"
	"time"

	"forgejo.coilysiren.me/coilyco-flight-deck/umbra/pkg/policy"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// clientName identifies umbra to the upstream during initialize. Servers log it
// and some gate behaviour on it, so it stays stable across versions.
const clientName = "umbra"

// DefaultTimeout bounds a single session's whole lifetime when the caller
// supplies no deadline, so a silent stdio server cannot hang the CLI.
const DefaultTimeout = 60 * time.Second

// terminateGrace bounds Close's wait for a stdio child to exit before SIGTERM.
// The SDK's 5s default costs that per call. See docs/mcpverb.md.
const terminateGrace = 100 * time.Millisecond

// Server is one fully declared upstream. Exactly one transport is set, which
// Validate enforces: an under-declared server is a policy hole, not a default.
type Server struct {
	// Name is the guardfile's name for this upstream, used only in errors.
	Name string

	Stdio *Stdio
	HTTP  *HTTPEndpoint
}

// Stdio is the subprocess transport: a command umbra starts and speaks
// newline-delimited JSON to over its stdin and stdout.
type Stdio struct {
	Command string
	Argv    []string
	// Env are `NAME=VALUE` overrides the caller already resolved. A secret reaches
	// the child here rather than through argv, which any local process can read.
	Env []string
}

// HTTPEndpoint is the Streamable HTTP transport.
type HTTPEndpoint struct {
	URL     string
	Headers map[string]string
	Client  *http.Client
}

// Validate refuses a server that is under- or over-declared, and puts a stdio
// command and argv through pkg/policy: a spawn gets no weaker gate here.
func (s Server) Validate() error {
	switch {
	case s.Stdio == nil && s.HTTP == nil:
		return fmt.Errorf("mcpclient: server %q declares no transport; add `mcp stdio { ... }` or `mcp http { ... }`", s.Name)
	case s.Stdio != nil && s.HTTP != nil:
		return fmt.Errorf("mcpclient: server %q declares both stdio and http; a server speaks one transport", s.Name)
	}
	if s.HTTP != nil {
		if s.HTTP.URL == "" {
			return fmt.Errorf("mcpclient: server %q has an http transport with no url", s.Name)
		}
		return nil
	}
	if s.Stdio.Command == "" {
		return fmt.Errorf("mcpclient: server %q has a stdio transport with no command", s.Name)
	}
	if err := policy.ValidateArg("command", s.Stdio.Command); err != nil {
		return fmt.Errorf("mcpclient: server %q: %w", s.Name, err)
	}
	if err := policy.ValidateArgSlice("argv", s.Stdio.Argv); err != nil {
		return fmt.Errorf("mcpclient: server %q: %w", s.Name, err)
	}
	return nil
}

// Session is a live client session over one upstream. Close it.
type Session struct {
	cs *mcp.ClientSession
}

// Connect validates the declaration, starts or dials the transport, and
// completes the initialize handshake.
func Connect(ctx context.Context, s Server) (*Session, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	sess, err := connect(ctx, s.transport(ctx))
	if err != nil {
		return nil, fmt.Errorf("mcpclient: connect %s: %w", s.label(), err)
	}
	return sess, nil
}

// connect completes the handshake over an already-built transport. Separate
// from Connect so a test drives an in-memory transport without a subprocess.
func connect(ctx context.Context, t mcp.Transport) (*Session, error) {
	client := mcp.NewClient(&mcp.Implementation{Name: clientName, Version: "v0"}, nil)
	cs, err := client.Connect(ctx, t, nil)
	if err != nil {
		return nil, err
	}
	return &Session{cs: cs}, nil
}

// transport builds the SDK transport for the declared shape.
func (s Server) transport(ctx context.Context) mcp.Transport {
	if s.HTTP != nil {
		return &mcp.StreamableClientTransport{
			Endpoint:   s.HTTP.URL,
			HTTPClient: s.httpClient(),
		}
	}
	cmd := exec.CommandContext(ctx, s.Stdio.Command, s.Stdio.Argv...) //nolint:gosec // argv is guardfile-declared and policy-validated above
	if len(s.Stdio.Env) > 0 {
		cmd.Env = append(cmd.Environ(), s.Stdio.Env...)
	}
	return &mcp.CommandTransport{Command: cmd, TerminateDuration: terminateGrace}
}

// httpClient returns the caller's client, or one carrying the declared headers.
func (s Server) httpClient() *http.Client {
	base := s.HTTP.Client
	if base == nil {
		base = &http.Client{Timeout: DefaultTimeout}
	}
	if len(s.HTTP.Headers) == 0 {
		return base
	}
	clone := *base
	clone.Transport = &headerTransport{base: base.Transport, headers: s.HTTP.Headers}
	return &clone
}

// headerTransport adds the declared headers to every request. The SDK owns the
// request construction, so auth attaches here rather than at the call site.
type headerTransport struct {
	base    http.RoundTripper
	headers map[string]string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	// Cloning keeps the caller's request untouched, which RoundTripper requires.
	out := req.Clone(req.Context())
	for k, v := range t.headers {
		out.Header.Set(k, v)
	}
	return base.RoundTrip(out)
}

// label names the upstream in an error without leaking a resolved secret.
func (s Server) label() string {
	switch {
	case s.HTTP != nil:
		return fmt.Sprintf("%s (%s)", s.Name, s.HTTP.URL)
	case s.Stdio != nil:
		return fmt.Sprintf("%s (%s)", s.Name, s.Stdio.Command)
	default:
		return s.Name
	}
}

// Close ends the session, terminating a stdio child.
func (s *Session) Close() error {
	if s == nil || s.cs == nil {
		return nil
	}
	return s.cs.Close()
}

// Tool is one upstream tool as umbra records it. It is umbra's own type rather
// than the SDK's so the lock format does not move when the SDK does.
type Tool struct {
	Name         string         `json:"name"`
	Title        string         `json:"title,omitempty"`
	Description  string         `json:"description,omitempty"`
	InputSchema  map[string]any `json:"inputSchema,omitempty"`
	OutputSchema map[string]any `json:"outputSchema,omitempty"`
	Annotations  map[string]any `json:"annotations,omitempty"`
	Meta         map[string]any `json:"_meta,omitempty"`
}

// ListTools walks every page and returns the full surface sorted by name,
// because tools/list order is unspecified and this result reaches a lock.
func (s *Session) ListTools(ctx context.Context) ([]Tool, error) {
	var tools []Tool
	for tool, err := range s.cs.Tools(ctx, nil) {
		if err != nil {
			return nil, fmt.Errorf("mcpclient: list tools: %w", err)
		}
		tools = append(tools, convertTool(tool))
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return tools, nil
}

// convertTool re-marshals the SDK's tool into umbra's. InputSchema arrives as
// `any`, so it round-trips through JSON rather than a lossy type assertion.
func convertTool(t *mcp.Tool) Tool {
	out := Tool{
		Name:        t.Name,
		Title:       t.Title,
		Description: t.Description,
		Meta:        t.Meta,
	}
	out.InputSchema = asObject(t.InputSchema)
	out.OutputSchema = asObject(t.OutputSchema)
	out.Annotations = asObject(t.Annotations)
	return out
}

// asObject renders any JSON-marshalable value as a generic object, or nil when
// it is not one. A non-object schema binds no flags, which mcpverb reports.
func asObject(v any) map[string]any {
	if v == nil {
		return nil
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil
	}
	return m
}

// Result is one tools/call outcome for the guard floor above: Decoded feeds a
// fail-when postcondition, Raw the renderer, IsError a tool-reported failure.
type Result struct {
	Decoded any
	Raw     []byte
	IsError bool
}

// CallTool fires one tool with already-bound arguments.
func (s *Session) CallTool(ctx context.Context, name string, args map[string]any) (Result, error) {
	res, err := s.cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return Result{}, fmt.Errorf("mcpclient: call %s: %w", name, err)
	}
	decoded := decodeResult(res)
	raw, err := json.Marshal(decoded)
	if err != nil {
		return Result{}, fmt.Errorf("mcpclient: encode %s result: %w", name, err)
	}
	return Result{Decoded: decoded, Raw: raw, IsError: res.IsError}, nil
}

// decodeResult picks the most structured form the result offers. A server that
// sets StructuredContent means it, so it wins over the same value as text.
func decodeResult(res *mcp.CallToolResult) any {
	if res.StructuredContent != nil {
		return res.StructuredContent
	}
	texts := make([]any, 0, len(res.Content))
	allText := true
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			texts = append(texts, decodeText(tc.Text))
			continue
		}
		allText = false
		break
	}
	switch {
	case allText && len(texts) == 1:
		return texts[0]
	case allText && len(texts) > 0:
		return texts
	}
	// A non-text block (image, audio, embedded resource) has no useful scalar
	// form, so the content list goes out as the protocol shaped it.
	return res.Content
}

// decodeText re-decodes a text block that is itself JSON, which a server with
// no output schema commonly returns. Left as a string, every guard is substring.
func decodeText(text string) any {
	var v any
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		return text
	}
	switch v.(type) {
	case map[string]any, []any:
		return v
	}
	// A bare number or bool that happens to parse stays the text it was.
	return text
}

// ReadResource reads one resource URI. This serves a `ui://` MCP Apps resource,
// which is an ordinary resources/read against the same session.
func (s *Session) ReadResource(ctx context.Context, uri string) (*mcp.ReadResourceResult, error) {
	res, err := s.cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri})
	if err != nil {
		return nil, fmt.Errorf("mcpclient: read resource %s: %w", uri, err)
	}
	return res, nil
}
