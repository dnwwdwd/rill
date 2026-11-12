package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"rill/internal/version"
)

type headerRoundTripper struct {
	base http.RoundTripper
	auth string
}

func (h headerRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	request = request.Clone(request.Context())
	request.Header.Set("Authorization", h.auth)
	return h.base.RoundTrip(request)
}

func mcpPost(t *testing.T, client *testClient, token string, payload map[string]any, wantStatus int) map[string]any {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, client.base+"/mcp", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := client.http.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != wantStatus {
		t.Fatalf("mcp status %d, want %d: %s", response.StatusCode, wantStatus, data)
	}
	if wantStatus != http.StatusOK {
		return nil
	}
	// Streamable HTTP defaults to SSE; accept a plain JSON body as well.
	raw := strings.TrimSpace(string(data))
	if strings.HasPrefix(raw, "{") {
		var result map[string]any
		if err := json.Unmarshal([]byte(raw), &result); err != nil {
			t.Fatalf("decode MCP JSON: %v: %s", err, data)
		}
		return result
	}
	var result map[string]any
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		if err := json.Unmarshal([]byte(payload), &result); err != nil {
			t.Fatalf("decode MCP SSE: %v: %s", err, payload)
		}
	}
	if result == nil {
		t.Fatalf("empty MCP response: %s", data)
	}
	return result
}

func TestMCPBearerAuthToolsAndTokenLifecycle(t *testing.T) {
	app, client := newTestApp(t, true)
	client.login()

	// 默认关闭。
	mcpPost(t, client, "", map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/list",
	}, http.StatusNotFound)

	client.decode(http.MethodPatch, "/api/v1/settings", map[string]any{
		"mcp": map[string]any{"enabled": true},
	}, true, http.StatusOK, nil)

	mcpPost(t, client, "", map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/list",
	}, http.StatusUnauthorized)
	mcpPost(t, client, "rill_mcp_wrong", map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/list",
	}, http.StatusUnauthorized)

	var created MCPTokenCreated
	client.decode(http.MethodPost, "/api/v1/mcp/tokens", map[string]any{"name": "测试 Agent"}, true, http.StatusCreated, &created)
	if !strings.HasPrefix(created.Token, mcpTokenPrefix) || created.Masked == created.Token {
		t.Fatalf("unexpected token payload: %+v", created)
	}
	var tokens []MCPToken
	client.decode(http.MethodGet, "/api/v1/mcp/tokens", nil, false, http.StatusOK, &tokens)
	if len(tokens) != 1 || tokens[0].Name != "测试 Agent" || tokens[0].Masked != created.Masked {
		t.Fatalf("token list: %+v", tokens)
	}

	list := mcpPost(t, client, created.Token, map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "tools/list",
	}, http.StatusOK)
	result, _ := list["result"].(map[string]any)
	tools, _ := result["tools"].([]any)
	if len(tools) < 10 {
		t.Fatalf("tools/list returned %d tools: %+v", len(tools), list)
	}
	names := map[string]bool{}
	for _, entry := range tools {
		tool, _ := entry.(map[string]any)
		name, _ := tool["name"].(string)
		names[name] = true
		if strings.Contains(strings.ToLower(name), "delete") {
			t.Fatalf("MCP must not expose delete tool %q", name)
		}
	}
	for _, want := range []string{"list_sources", "list_items", "get_item", "create_source", "update_item_state", "create_filter_rule", "create_digest_rule"} {
		if !names[want] {
			t.Fatalf("missing tool %q in %v", want, names)
		}
	}

	call := mcpPost(t, client, created.Token, map[string]any{
		"jsonrpc": "2.0", "id": 3, "method": "tools/call",
		"params": map[string]any{"name": "create_folder", "arguments": map[string]any{"name": "MCP 创建"}},
	}, http.StatusOK)
	callResult, _ := call["result"].(map[string]any)
	content, _ := callResult["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("empty tool result: %+v", call)
	}
	text, _ := content[0].(map[string]any)["text"].(string)
	var folder Folder
	if err := json.Unmarshal([]byte(text), &folder); err != nil {
		t.Fatalf("tool result is not folder JSON: %v: %s", err, text)
	}
	if folder.ID == 0 || folder.Name != "MCP 创建" {
		t.Fatalf("created folder: %+v", folder)
	}

	client.decode(http.MethodGet, "/api/v1/mcp/tokens", nil, false, http.StatusOK, &tokens)
	if len(tokens) != 1 || tokens[0].LastUsedAt == nil || *tokens[0].LastUsedAt == "" {
		t.Fatalf("token last_used_at not updated: %+v", tokens)
	}

	client.decode(http.MethodDelete, "/api/v1/mcp/tokens/"+strconv.FormatInt(created.ID, 10), nil, true, http.StatusNoContent, nil)
	mcpPost(t, client, created.Token, map[string]any{
		"jsonrpc": "2.0", "id": 4, "method": "tools/list",
	}, http.StatusUnauthorized)

	if _, err := app.db.ExecContext(context.Background(), `SELECT 1`); err != nil {
		t.Fatal(err)
	}
}

func TestMCPOfficialClientHandshake(t *testing.T) {
	_, client := newTestApp(t, true)
	client.login()
	client.decode(http.MethodPatch, "/api/v1/settings", map[string]any{
		"mcp": map[string]any{"enabled": true},
	}, true, http.StatusOK, nil)
	var created MCPTokenCreated
	client.decode(http.MethodPost, "/api/v1/mcp/tokens", map[string]any{"name": "官方客户端"}, true, http.StatusCreated, &created)

	httpClient := &http.Client{Transport: headerRoundTripper{
		base: http.DefaultTransport,
		auth: "Bearer " + created.Token,
	}}
	transport := &mcp.StreamableClientTransport{
		Endpoint:   client.base + "/mcp",
		HTTPClient: httpClient,
		MaxRetries: -1,
	}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "rill-test", Version: "1.0.0"}, nil).
		Connect(context.Background(), transport, nil)
	if err != nil {
		t.Fatalf("connect MCP client: %v", err)
	}
	defer session.Close()
	if info := session.InitializeResult(); info == nil || info.ServerInfo == nil || info.ServerInfo.Version != version.Version {
		t.Fatalf("MCP server version = %+v, want %s", info, version.Version)
	}

	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	if len(tools.Tools) < 10 {
		t.Fatalf("only %d tools", len(tools.Tools))
	}
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "list_folders",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("call list_folders: %v", err)
	}
	if result.IsError || len(result.Content) == 0 {
		t.Fatalf("list_folders result: %+v", result)
	}
}
