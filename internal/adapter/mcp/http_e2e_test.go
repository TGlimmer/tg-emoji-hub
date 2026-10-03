package mcp

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcpserver "github.com/mark3labs/mcp-go/server"
)

// TestStreamableHTTP_InitializeFrame 拿 NewStreamableHTTPServer 起 httptest server，
// 发一帧 initialize JSON-RPC，验证 MCP HTTP 传输层通了。
//
// 不验证完整 tool 调用（那是单 handler test 的职责），仅证明 transport 没坏。
func TestStreamableHTTP_InitializeFrame(t *testing.T) {
	deps := setupMCPDeps(t)
	mcpSrv := NewServer(deps)
	httpHandler := mcpserver.NewStreamableHTTPServer(mcpSrv)

	ts := httptest.NewServer(httpHandler)
	defer ts.Close()

	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0.0.1"}}}`)
	req, err := http.NewRequest(http.MethodPost, ts.URL, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build req: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// Streamable HTTP 要求 Accept 同时含 application/json 和 text/event-stream
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http err: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d body=%s", resp.StatusCode, raw)
	}
	raw, _ := io.ReadAll(resp.Body)
	text := string(raw)
	if !strings.Contains(text, `"jsonrpc":"2.0"`) {
		t.Errorf("响应不是 JSON-RPC 格式: %s", text)
	}
	// initialize 响应必含 serverInfo
	if !strings.Contains(text, "serverInfo") && !strings.Contains(text, "protocolVersion") {
		t.Errorf("响应缺 serverInfo/protocolVersion: %s", text)
	}
}
