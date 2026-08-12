package specverb

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"forgejo.coilysiren.me/coilyco-flight-deck/cli-guard/http/guardfile"
)

// readSpec loads a testdata spec by filename.
func readSpec(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return raw
}

// TestDetectSpecVersion proves the version sniffer routes Swagger 2.0 JSON,
// OpenAPI 3.0 JSON, and OpenAPI 3.1 YAML to the right reader.
func TestDetectSpecVersion(t *testing.T) {
	cases := map[string]int{
		"forgejo.swagger.v1.json": 2,
		"trello.openapi.json":     3,
		"tailscale.openapi.yaml":  3,
	}
	for name, want := range cases {
		got, err := detectSpecVersion(readSpec(t, name))
		if err != nil {
			t.Errorf("detectSpecVersion(%s): %v", name, err)
			continue
		}
		if got != want {
			t.Errorf("detectSpecVersion(%s) = %d, want %d", name, got, want)
		}
	}
}

// TestOpenAPI3TrelloReadsQueryPayload proves the 3.0 JSON reader resolves an op
// whose create/update fields ride in `in: query` params (Trello's shape).
func TestOpenAPI3TrelloReadsQueryPayload(t *testing.T) {
	spec, err := parseOpenAPI3(readSpec(t, "trello.openapi.json"))
	if err != nil {
		t.Fatalf("parseOpenAPI3: %v", err)
	}
	method, path, op, err := spec.findOp("post-cards")
	if err != nil {
		t.Fatalf("findOp post-cards: %v", err)
	}
	if method != "POST" || path != "/cards" {
		t.Errorf("post-cards = %s %s, want POST /cards", method, path)
	}
	q := map[string]bool{}
	for _, p := range queryParamsOf(op) {
		q[p.Name] = true
	}
	for _, want := range []string{"name", "idList", "desc"} {
		if !q[want] {
			t.Errorf("post-cards missing query param %q (q=%v)", want, q)
		}
	}

	// get-boards-id carries a path param plus scalar query params.
	_, gpath, gop, err := spec.findOp("get-boards-id")
	if err != nil {
		t.Fatalf("findOp get-boards-id: %v", err)
	}
	if gpath != "/boards/{id}" {
		t.Errorf("get-boards-id path = %q", gpath)
	}
	if got := pathParamsInOrder(gpath); len(got) != 1 || got[0] != "id" {
		t.Errorf("get-boards-id path params = %v, want [id]", got)
	}
	if len(queryParamsOf(gop)) == 0 {
		t.Error("get-boards-id should promote scalar query params")
	}
}

// TestOpenAPI3TailscaleResolvesComponentParams proves the 3.1 YAML reader resolves
// path params declared as $refs into components/parameters and reads a JSON body.
func TestOpenAPI3TailscaleResolvesComponentParams(t *testing.T) {
	spec, err := parseOpenAPI3(readSpec(t, "tailscale.openapi.yaml"))
	if err != nil {
		t.Fatalf("parseOpenAPI3: %v", err)
	}
	// listTailnetDevices draws its only path param from a path-level $ref param.
	_, path, _, err := spec.findOp("listTailnetDevices")
	if err != nil {
		t.Fatalf("findOp listTailnetDevices: %v", err)
	}
	if got := pathParamsInOrder(path); len(got) != 1 || got[0] != "tailnet" {
		t.Errorf("listTailnetDevices path params = %v, want [tailnet]", got)
	}

	// setPolicyFile's request body comes from requestBody.content (json + hujson).
	_, _, op, err := spec.findOp("setPolicyFile")
	if err != nil {
		t.Fatalf("findOp setPolicyFile: %v", err)
	}
	if _, ok := spec.bodySchema(op); !ok {
		t.Error("setPolicyFile should expose a request body schema")
	}
}

// TestPruneOpenAPI3 proves an OpenAPI 3.x spec prunes to only the granted ops and
// their reachable components, re-emits JSON, parses back, and is idempotent.
func TestPruneOpenAPI3(t *testing.T) {
	full := readSpec(t, "tailscale.openapi.yaml")
	gf, err := guardfile.Parse([]byte(`wrap ward ops tailscale {
		spec tailscale.openapi.yaml
		auth bearer { value ssm "/tailscale/api-key" }
		can list devices { op "listTailnetDevices" }
		can create keys { op "createKey" }
	}`))
	if err != nil {
		t.Fatalf("parse guardfile: %v", err)
	}
	pruned, err := Prune(full, gf)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if len(pruned) >= len(full) {
		t.Errorf("pruned (%d) not smaller than full (%d)", len(pruned), len(full))
	}
	var doc struct {
		OpenAPI string                                `json:"openapi"`
		Paths   map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(pruned, &doc); err != nil {
		t.Fatalf("pruned spec is not valid json: %v", err)
	}
	if _, ok := doc.Paths["/tailnet/{tailnet}/devices"]; !ok {
		t.Errorf("pruned spec missing the granted devices path; got %v", keysOf(doc.Paths))
	}
	if _, ok := doc.Paths["/device/{deviceId}"]; ok {
		t.Error("pruned spec kept an ungranted path")
	}
	// The pruned lock parses back through the engine and is idempotent.
	if _, err := parseSwagger(pruned); err != nil {
		t.Fatalf("pruned spec failed to parse: %v", err)
	}
	twice, err := Prune(pruned, gf)
	if err != nil {
		t.Fatalf("Prune (second): %v", err)
	}
	if string(pruned) != string(twice) {
		t.Error("OpenAPI-3 Prune is not idempotent")
	}
}

// TestBuildOpenAPI3Tailscale proves the engine mounts a guarded tree off a 3.1
// YAML spec end to end: path params, a JSON-body create, the describe surface.
func TestBuildOpenAPI3Tailscale(t *testing.T) {
	spec := readSpec(t, "tailscale.openapi.yaml")
	gf, err := guardfile.Parse([]byte(`wrap ward ops tailscale {
		spec tailscale.openapi.yaml
		base-url "https://api.tailscale.com/api/v2"
		auth bearer { value ssm "/tailscale/api-key" }
		can list devices { op "listTailnetDevices" }
		can create keys { op "createKey" }
	}`))
	if err != nil {
		t.Fatalf("parse guardfile: %v", err)
	}
	surface, err := Describe(Config{Guardfile: gf, Spec: spec})
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	byLeaf := map[string]VerbInfo{}
	for _, v := range surface.Verbs {
		byLeaf[v.Leaf] = v
	}
	if got := byLeaf["list"]; got.Path != "/tailnet/{tailnet}/devices" || got.Method != "GET" {
		t.Errorf("list devices = %s %s", got.Method, got.Path)
	}
	if got := byLeaf["create"]; got.Method != "POST" {
		t.Errorf("create keys method = %s, want POST", got.Method)
	}
}

// TestRawResponseOpReadsDeclaredMediaType proves the engine decides raw-vs-parsed
// from the spec rather than from the bytes that come back. Forgejo's Actions job
// log is the motivating case: it declares text/plain, and parsing it as JSON
// fails on the first timestamp.
func TestRawResponseOpReadsDeclaredMediaType(t *testing.T) {
	cases := map[string]struct {
		media string
		want  bool
	}{
		"plain text is raw":    {media: "text/plain", want: true},
		"zip is raw":           {media: "application/zip", want: true},
		"octet-stream is raw":  {media: "application/octet-stream", want: true},
		"json is parsed":       {media: "application/json", want: false},
		"suffixed json parsed": {media: "application/vnd.api+json", want: false},
		"json with params":     {media: "application/json; charset=utf-8", want: false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			op := &openapi3.Operation{Responses: openapi3.NewResponses()}
			resp := openapi3.NewResponse().WithContent(openapi3.Content{
				tc.media: openapi3.NewMediaType(),
			})
			op.Responses.Set("200", &openapi3.ResponseRef{Value: resp})
			if got := rawResponseOp(op); got != tc.want {
				t.Errorf("rawResponseOp(%s) = %v, want %v", tc.media, got, tc.want)
			}
		})
	}
}

// TestRawResponseOpDefaultsToParsed keeps the fail-safe direction: a spec that
// says nothing about its response keeps the parsed path it has always had.
func TestRawResponseOpDefaultsToParsed(t *testing.T) {
	if rawResponseOp(nil) {
		t.Error("a nil operation must not be treated as raw")
	}
	if rawResponseOp(&openapi3.Operation{}) {
		t.Error("an operation with no responses must not be treated as raw")
	}
	op := &openapi3.Operation{Responses: openapi3.NewResponses()}
	op.Responses.Set("404", &openapi3.ResponseRef{Value: openapi3.NewResponse().WithContent(
		openapi3.Content{"text/plain": openapi3.NewMediaType()},
	)})
	if rawResponseOp(op) {
		t.Error("a non-success response must not decide the success path")
	}
}
