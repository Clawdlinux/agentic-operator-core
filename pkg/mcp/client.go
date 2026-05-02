/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// MCPClient is a tool-agnostic client for calling MCP servers
type MCPClient struct {
	endpoint string
	client   *http.Client
}

// ToolRequest is the payload sent to the MCP server
type ToolRequest struct {
	Tool   string                 `json:"tool"`
	Params map[string]interface{} `json:"params"`
}

// ToolResponse is the response from the MCP server
type ToolResponse struct {
	Tool    string                 `json:"tool"`
	Result  map[string]interface{} `json:"result,omitempty"`
	Error   string                 `json:"error,omitempty"`
	Success bool                   `json:"success"`
}

// ToolListResponse is the legacy response shape for listing tools — a flat
// list of tool names. Retained for backwards compat with the MockServer
// emitter; new servers (`agentctl mcp serve`) emit ToolDescriptor objects.
// MCPClient.ListTools accepts both shapes via toolListEntry below.
type ToolListResponse struct {
	Tools []string `json:"tools"`
}

// toolListResponseFlex decodes either the legacy `[]string` shape or the new
// `[]ToolDescriptor` shape into a single Names slice. Used internally by
// MCPClient.ListTools.
type toolListResponseFlex struct {
	Tools []toolListEntry `json:"tools"`
}

type toolListEntry struct {
	Name string
}

// UnmarshalJSON accepts both `"name"` (string) and `{"name": "name", ...}`
// (object) shapes.
func (t *toolListEntry) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		t.Name = s
		return nil
	}
	var obj struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	t.Name = obj.Name
	return nil
}

// NewMCPClient creates a new MCP client for the given endpoint
func NewMCPClient(endpoint string) *MCPClient {
	return &MCPClient{
		endpoint: endpoint,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ListTools queries the MCP server for available tools
func (c *MCPClient) ListTools() ([]string, error) {
	resp, err := c.client.Get(fmt.Sprintf("%s/tools", c.endpoint))
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("MCP server returned status %d: %s", resp.StatusCode, string(body))
	}

	var toolResp toolListResponseFlex
	if err := json.NewDecoder(resp.Body).Decode(&toolResp); err != nil {
		return nil, fmt.Errorf("failed to decode tool list: %w", err)
	}

	names := make([]string, 0, len(toolResp.Tools))
	for _, t := range toolResp.Tools {
		names = append(names, t.Name)
	}
	return names, nil
}

// CallTool calls a specific tool on the MCP server with the given parameters
func (c *MCPClient) CallTool(toolName string, params map[string]interface{}) (map[string]interface{}, error) {
	req := ToolRequest{
		Tool:   toolName,
		Params: params,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.client.Post(
		fmt.Sprintf("%s/call_tool", c.endpoint),
		"application/json",
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to call tool: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("MCP server returned status %d: %s", resp.StatusCode, string(body))
	}

	var toolResp ToolResponse
	if err := json.NewDecoder(resp.Body).Decode(&toolResp); err != nil {
		return nil, fmt.Errorf("failed to decode tool response: %w", err)
	}

	if !toolResp.Success {
		return nil, fmt.Errorf("tool execution failed: %s", toolResp.Error)
	}

	return toolResp.Result, nil
}
