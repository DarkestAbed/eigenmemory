package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/javi/eigenmemory/internal/config"
	"github.com/javi/eigenmemory/internal/core"
	"github.com/javi/eigenmemory/internal/types"
)

func TestInitialize(t *testing.T) {
	tmp := t.TempDir()
	if err := config.SaveConfig(config.PathsFor(tmp), config.Default("test")); err != nil {
		t.Fatal(err)
	}

	store, err := core.OpenAt(tmp)
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	defer func() { _ = store.Close() }()

	stdin := &bytes.Buffer{}
	stdoutR, stdoutW := io.Pipe()

	server := newTestServer(store, stdin, stdoutW)
	RegisterWikiTools(server)
	RegisterWikiResources(server)

	req := Request{
		JSONRPC: "2.0",
		ID:      rawJSON(1),
		Method:  "initialize",
		Params: rawJSON(InitializeParams{
			ProtocolVersion: "2024-11-05",
			ClientInfo:      Implementation{Name: "test", Version: "0.1"},
		}),
	}
	if err := send(stdin, req); err != nil {
		t.Fatal(err)
	}

	go func() { _ = server.Run() }()

	resp, err := readResponse(stdoutR)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("initialize error: %v", resp.Error)
	}

	var result InitializeResult
	b, _ := json.Marshal(resp.Result)
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.ServerInfo.Name != "eigenmemory" {
		t.Errorf("server name = %q, want eigenmemory", result.ServerInfo.Name)
	}
}

func TestToolsList(t *testing.T) {
	tmp := t.TempDir()
	if err := config.SaveConfig(config.PathsFor(tmp), config.Default("test")); err != nil {
		t.Fatal(err)
	}
	store, err := core.OpenAt(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	stdin := &bytes.Buffer{}
	stdoutR, stdoutW := io.Pipe()

	server := newTestServer(store, stdin, stdoutW)
	RegisterWikiTools(server)

	if err := send(stdin, Request{JSONRPC: "2.0", ID: rawJSON(1), Method: "initialize"}); err != nil {
		t.Fatal(err)
	}
	if err := send(stdin, Request{JSONRPC: "2.0", ID: rawJSON(2), Method: "tools/list"}); err != nil {
		t.Fatal(err)
	}

	go func() { _ = server.Run() }()

	// Skip initialize response.
	if _, err := readResponse(stdoutR); err != nil {
		t.Fatalf("init response: %v", err)
	}

	resp, err := readResponse(stdoutR)
	if err != nil {
		t.Fatalf("tools/list response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("tools/list error: %v", resp.Error)
	}

	var result ToolsListResult
	b, _ := json.Marshal(resp.Result)
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.Tools) != 7 {
		t.Errorf("tool count = %d, want 7", len(result.Tools))
	}
}

func TestWikiRecall(t *testing.T) {
	tmp := t.TempDir()
	if err := config.SaveConfig(config.PathsFor(tmp), config.Default("test")); err != nil {
		t.Fatal(err)
	}
	store, err := core.OpenAt(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	page := storePage(t, store, "auth-service", "entity", "# Auth Service\n\nHandles login.")

	stdin := &bytes.Buffer{}
	stdoutR, stdoutW := io.Pipe()

	server := newTestServer(store, stdin, stdoutW)
	RegisterWikiTools(server)

	if err := send(stdin, Request{JSONRPC: "2.0", ID: rawJSON(1), Method: "initialize"}); err != nil {
		t.Fatal(err)
	}

	call := ToolCallParams{
		Name:      "wiki_recall",
		Arguments: rawJSON(map[string]any{"query": "login", "limit": 5}),
	}
	if err := send(stdin, Request{JSONRPC: "2.0", ID: rawJSON(2), Method: "tools/call", Params: rawJSON(call)}); err != nil {
		t.Fatal(err)
	}

	go func() { _ = server.Run() }()

	// Skip initialize response.
	if _, err := readResponse(stdoutR); err != nil {
		t.Fatalf("init response: %v", err)
	}

	resp, err := readResponse(stdoutR)
	if err != nil {
		t.Fatalf("wiki_recall response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("wiki_recall error: %v", resp.Error)
	}

	var result ToolCallResult
	b, _ := json.Marshal(resp.Result)
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("content count = %d, want 1", len(result.Content))
	}
	if !strings.Contains(result.Content[0].Text, page.Slug) {
		t.Errorf("recall result missing slug: %q", result.Content[0].Text)
	}
}

func TestResourcesRead(t *testing.T) {
	tmp := t.TempDir()
	if err := config.SaveConfig(config.PathsFor(tmp), config.Default("test")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "wiki"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "wiki", "index.md"), []byte("# Test Index\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := core.OpenAt(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	stdin := &bytes.Buffer{}
	stdoutR, stdoutW := io.Pipe()

	server := newTestServer(store, stdin, stdoutW)
	RegisterWikiResources(server)

	if err := send(stdin, Request{JSONRPC: "2.0", ID: rawJSON(1), Method: "initialize"}); err != nil {
		t.Fatal(err)
	}
	readCall := ResourceReadParams{URI: "memory://index"}
	if err := send(stdin, Request{JSONRPC: "2.0", ID: rawJSON(2), Method: "resources/read", Params: rawJSON(readCall)}); err != nil {
		t.Fatal(err)
	}

	go func() { _ = server.Run() }()

	// Skip initialize response.
	if _, err := readResponse(stdoutR); err != nil {
		t.Fatalf("init response: %v", err)
	}

	resp, err := readResponse(stdoutR)
	if err != nil {
		t.Fatalf("resources/read response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("resources/read error: %v", resp.Error)
	}

	var result ResourceReadResult
	b, _ := json.Marshal(resp.Result)
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.Contents) != 1 {
		t.Fatalf("contents count = %d, want 1", len(result.Contents))
	}
	if !strings.Contains(result.Contents[0].Text, "Test Index") {
		t.Errorf("resource missing expected text: %q", result.Contents[0].Text)
	}
}

func TestWikiRemember(t *testing.T) {
	tmp := t.TempDir()
	if err := config.SaveConfig(config.PathsFor(tmp), config.Default("test")); err != nil {
		t.Fatal(err)
	}
	store, err := core.OpenAt(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	stdin := &bytes.Buffer{}
	stdoutR, stdoutW := io.Pipe()

	server := newTestServer(store, stdin, stdoutW)
	RegisterWikiTools(server)

	if err := send(stdin, Request{JSONRPC: "2.0", ID: rawJSON(1), Method: "initialize"}); err != nil {
		t.Fatal(err)
	}

	call := ToolCallParams{
		Name: "wiki_remember",
		Arguments: rawJSON(map[string]any{
			"fact":      "We decided to migrate to pnpm for faster installs.",
			"page_type": "project",
			"slug":      "package-manager-decision",
			"title":     "Package Manager Decision",
			"tags":      []string{"package-manager", "decision"},
		}),
	}
	if err := send(stdin, Request{JSONRPC: "2.0", ID: rawJSON(2), Method: "tools/call", Params: rawJSON(call)}); err != nil {
		t.Fatal(err)
	}

	go func() { _ = server.Run() }()

	// Skip initialize response.
	if _, err := readResponse(stdoutR); err != nil {
		t.Fatalf("init response: %v", err)
	}

	resp, err := readResponse(stdoutR)
	if err != nil {
		t.Fatalf("wiki_remember response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("wiki_remember error: %v", resp.Error)
	}

	var result ToolCallResult
	b, _ := json.Marshal(resp.Result)
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("content count = %d, want 1", len(result.Content))
	}
	if !strings.Contains(result.Content[0].Text, "project/package-manager-decision.md") {
		t.Errorf("remember result missing path: %q", result.Content[0].Text)
	}

	// Verify it is searchable.
	hits, err := store.Search.Search("pnpm", 5)
	if err != nil {
		t.Fatalf("search after remember: %v", err)
	}
	if len(hits) != 1 {
		t.Errorf("hits after remember = %d, want 1", len(hits))
	}
}

func TestToolCall_RejectedBeforeInitialize(t *testing.T) {
	tmp := t.TempDir()
	if err := config.SaveConfig(config.PathsFor(tmp), config.Default("test")); err != nil {
		t.Fatal(err)
	}
	store, err := core.OpenAt(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	stdin := &bytes.Buffer{}
	stdoutR, stdoutW := io.Pipe()

	server := newTestServer(store, stdin, stdoutW)
	RegisterWikiTools(server)

	// No "initialize" call first.
	call := ToolCallParams{
		Name:      "wiki_recall",
		Arguments: rawJSON(map[string]any{"query": "anything"}),
	}
	if err := send(stdin, Request{JSONRPC: "2.0", ID: rawJSON(1), Method: "tools/call", Params: rawJSON(call)}); err != nil {
		t.Fatal(err)
	}

	go func() { _ = server.Run() }()

	resp, err := readResponse(stdoutR)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected an error for tools/call before initialize, got none")
	}
	if resp.Error.Code != ErrServerNotInitialized {
		t.Errorf("error code = %d, want %d", resp.Error.Code, ErrServerNotInitialized)
	}
}

func TestHandleRequest_NullIDGetsResponse(t *testing.T) {
	tmp := t.TempDir()
	if err := config.SaveConfig(config.PathsFor(tmp), config.Default("test")); err != nil {
		t.Fatal(err)
	}
	store, err := core.OpenAt(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	stdin := &bytes.Buffer{}
	stdoutR, stdoutW := io.Pipe()

	server := newTestServer(store, stdin, stdoutW)
	RegisterWikiTools(server)

	// A request with an explicit `"id": null` is NOT a notification per
	// JSON-RPC 2.0 and must still receive a response.
	raw := `{"jsonrpc":"2.0","id":null,"method":"initialize"}` + "\n"
	if _, err := stdin.WriteString(raw); err != nil {
		t.Fatal(err)
	}

	go func() { _ = server.Run() }()

	resp, err := readResponse(stdoutR)
	if err != nil {
		t.Fatalf("expected a response for an explicit null id, got read error: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestNegotiateProtocolVersion(t *testing.T) {
	cases := []struct {
		requested string
		want      string
	}{
		{"2024-11-05", "2024-11-05"},
		{"2025-06-18", "2025-06-18"},
		{"", supportedProtocolVersions[0]},
		{"1999-01-01", supportedProtocolVersions[0]},
	}
	for _, c := range cases {
		got := negotiateProtocolVersion(c.requested)
		if got != c.want {
			t.Errorf("negotiateProtocolVersion(%q) = %q, want %q", c.requested, got, c.want)
		}
	}
}

func TestWikiRemember_RejectsPathTraversalSlug(t *testing.T) {
	tmp := t.TempDir()
	if err := config.SaveConfig(config.PathsFor(tmp), config.Default("test")); err != nil {
		t.Fatal(err)
	}
	store, err := core.OpenAt(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	stdin := &bytes.Buffer{}
	stdoutR, stdoutW := io.Pipe()

	server := newTestServer(store, stdin, stdoutW)
	RegisterWikiTools(server)

	if err := send(stdin, Request{JSONRPC: "2.0", ID: rawJSON(1), Method: "initialize"}); err != nil {
		t.Fatal(err)
	}

	const escapeTarget = "/tmp/eigenmemory_mcp_poc.md"
	_ = os.Remove(escapeTarget)
	defer func() { _ = os.Remove(escapeTarget) }()

	call := ToolCallParams{
		Name: "wiki_remember",
		Arguments: rawJSON(map[string]any{
			"fact":      "pwned content from an MCP-level path traversal attempt",
			"page_type": "entity",
			"slug":      "../../../../../../tmp/eigenmemory_mcp_poc",
		}),
	}
	if err := send(stdin, Request{JSONRPC: "2.0", ID: rawJSON(2), Method: "tools/call", Params: rawJSON(call)}); err != nil {
		t.Fatal(err)
	}

	go func() { _ = server.Run() }()

	if _, err := readResponse(stdoutR); err != nil {
		t.Fatalf("init response: %v", err)
	}

	resp, err := readResponse(stdoutR)
	if err != nil {
		t.Fatalf("wiki_remember response: %v", err)
	}

	var result ToolCallResult
	b, _ := json.Marshal(resp.Result)
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected wiki_remember to reject a path-traversal slug, got success: %+v", result)
	}
	if _, err := os.Stat(escapeTarget); err == nil {
		t.Fatal("SECURITY REGRESSION: path-traversal slug escaped the wiki directory")
	}
}

// TestWikiRemember_SetsStatus is a regression test for C7: page lifecycle
// status was checked by lint but no tool could ever set it.
func TestWikiRemember_SetsStatus(t *testing.T) {
	tmp := t.TempDir()
	if err := config.SaveConfig(config.PathsFor(tmp), config.Default("test")); err != nil {
		t.Fatal(err)
	}
	store, err := core.OpenAt(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	stdin := &bytes.Buffer{}
	stdoutR, stdoutW := io.Pipe()

	server := newTestServer(store, stdin, stdoutW)
	RegisterWikiTools(server)

	if err := send(stdin, Request{JSONRPC: "2.0", ID: rawJSON(1), Method: "initialize"}); err != nil {
		t.Fatal(err)
	}
	call := ToolCallParams{
		Name: "wiki_remember",
		Arguments: rawJSON(map[string]any{
			"fact":      "This decision is now outdated.",
			"page_type": "project",
			"slug":      "old-decision",
			"status":    "stale",
		}),
	}
	if err := send(stdin, Request{JSONRPC: "2.0", ID: rawJSON(2), Method: "tools/call", Params: rawJSON(call)}); err != nil {
		t.Fatal(err)
	}

	go func() { _ = server.Run() }()

	if _, err := readResponse(stdoutR); err != nil {
		t.Fatalf("init response: %v", err)
	}
	resp, err := readResponse(stdoutR)
	if err != nil {
		t.Fatalf("wiki_remember response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("wiki_remember error: %v", resp.Error)
	}

	page, err := store.LoadPage(types.PageTypeProject, "old-decision")
	if err != nil {
		t.Fatalf("LoadPage: %v", err)
	}
	if page.Frontmatter.Status != types.PageStatusStale {
		t.Errorf("Status = %q, want %q", page.Frontmatter.Status, types.PageStatusStale)
	}
}

func TestWikiRemember_RejectsInvalidStatus(t *testing.T) {
	tmp := t.TempDir()
	if err := config.SaveConfig(config.PathsFor(tmp), config.Default("test")); err != nil {
		t.Fatal(err)
	}
	store, err := core.OpenAt(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	stdin := &bytes.Buffer{}
	stdoutR, stdoutW := io.Pipe()

	server := newTestServer(store, stdin, stdoutW)
	RegisterWikiTools(server)

	if err := send(stdin, Request{JSONRPC: "2.0", ID: rawJSON(1), Method: "initialize"}); err != nil {
		t.Fatal(err)
	}
	call := ToolCallParams{
		Name: "wiki_remember",
		Arguments: rawJSON(map[string]any{
			"fact":      "whatever",
			"page_type": "project",
			"slug":      "whatever",
			"status":    "not-a-real-status",
		}),
	}
	if err := send(stdin, Request{JSONRPC: "2.0", ID: rawJSON(2), Method: "tools/call", Params: rawJSON(call)}); err != nil {
		t.Fatal(err)
	}

	go func() { _ = server.Run() }()

	if _, err := readResponse(stdoutR); err != nil {
		t.Fatalf("init response: %v", err)
	}
	resp, err := readResponse(stdoutR)
	if err != nil {
		t.Fatalf("wiki_remember response: %v", err)
	}
	var result ToolCallResult
	b, _ := json.Marshal(resp.Result)
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected an error for an invalid status value")
	}
}

func newTestServer(store *core.Store, stdin io.Reader, stdout io.Writer) *Server {
	s := NewServer(store)
	s.reader = bufio.NewReader(stdin)
	s.writer = stdout
	return s
}

func send(w io.Writer, req Request) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	_, err = w.Write(append(data, '\n'))
	return err
}

func readResponse(r io.Reader) (Response, error) {
	reader := bufio.NewReader(r)
	line, err := reader.ReadString('\n')
	if err != nil {
		return Response{}, err
	}
	var resp Response
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		return Response{}, err
	}
	return resp, nil
}

func storePage(t *testing.T, store *core.Store, slug, pageType, body string) *types.Page {
	t.Helper()
	pt := types.PageType(pageType)
	p := &types.Page{
		Frontmatter: types.DefaultFrontmatter(pt),
		Slug:        slug,
		Body:        body,
	}
	if err := store.SavePage(p, pt); err != nil {
		t.Fatalf("SavePage: %v", err)
	}
	return p
}
