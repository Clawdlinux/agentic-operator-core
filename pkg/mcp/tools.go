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

// registeredTools returns the full set of tools surfaced by the MCP server.
// Schemas follow JSON Schema Draft-07 — the format Claude Desktop and Cursor
// expect for MCP tool discovery.
func registeredTools() []ToolDescriptor {
	stringProp := func(desc string) map[string]interface{} {
		return map[string]interface{}{"type": "string", "description": desc}
	}
	stringArrayProp := func(desc string) map[string]interface{} {
		return map[string]interface{}{
			"type":        "array",
			"description": desc,
			"items":       map[string]interface{}{"type": "string"},
		}
	}

	return []ToolDescriptor{
		{
			Name:        ToolCreateWorkload,
			Description: "Provision a new AgentWorkload in the cluster. Returns the assigned UID and initial Pending phase. Use this to spin up a fresh execution environment for a sub-task.",
			InputSchema: map[string]interface{}{
				"type":     "object",
				"required": []string{"name", "objective"},
				"properties": map[string]interface{}{
					"name":                 stringProp("AgentWorkload metadata.name. Must be a valid DNS-1123 label."),
					"namespace":            stringProp("Target namespace. Defaults to the operator's namespace."),
					"objective":            stringProp("High-level goal for the agent (1-1000 chars)."),
					"workloadType":         stringProp("One of: generic|ceph|minio|postgres|aws|kubernetes. Default: generic."),
					"workflowName":         stringProp("Registered workflow to execute. Default: research-swarm."),
					"mcpServerEndpoint":    stringProp("HTTPS endpoint of an upstream MCP server the workload should call (optional)."),
					"agents":               stringArrayProp("List of agent names to run."),
					"targetUrls":           stringArrayProp("URLs the workflow should process."),
					"autoApproveThreshold": stringProp("Confidence threshold for auto-approval, e.g. \"0.95\"."),
					"opaPolicy":            stringProp("strict|permissive — safety policy for action execution."),
				},
			},
		},
		{
			Name:        ToolGetWorkloadStatus,
			Description: "Get the current phase + workflow steps for an AgentWorkload. Cheap polling endpoint for an orchestrator agent waiting on completion.",
			InputSchema: map[string]interface{}{
				"type":     "object",
				"required": []string{"name"},
				"properties": map[string]interface{}{
					"name":      stringProp("AgentWorkload metadata.name."),
					"namespace": stringProp("Namespace. Defaults to the operator's namespace."),
				},
			},
		},
		{
			Name:        ToolListWorkloads,
			Description: "List AgentWorkloads in a namespace (or all namespaces if omitted). Returns name, phase, model, age, and today's cost per workload.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"namespace":     stringProp("Namespace to scope to. Empty = all namespaces."),
					"labelSelector": stringProp("Single key=value filter applied client-side (e.g. tenant=acme)."),
				},
			},
		},
		{
			Name:        ToolGetWorkloadLogs,
			Description: "Fetch the most recent log lines from the runtime pod backing this workload. Use this when the workload phase is Failed or stuck.",
			InputSchema: map[string]interface{}{
				"type":     "object",
				"required": []string{"name"},
				"properties": map[string]interface{}{
					"name": stringProp("AgentWorkload metadata.name."),
					"tail": map[string]interface{}{
						"type":        "integer",
						"description": "Number of lines to tail. Default 100.",
						"minimum":     1,
						"maximum":     5000,
					},
				},
			},
		},
		{
			Name:        ToolGetWorkloadCost,
			Description: "Return per-workload token + USD cost (today + month-to-date) sourced from the LiteLLM cost-attribution endpoint. Returns zeros when no records exist yet.",
			InputSchema: map[string]interface{}{
				"type":     "object",
				"required": []string{"name"},
				"properties": map[string]interface{}{
					"name":      stringProp("AgentWorkload metadata.name."),
					"namespace": stringProp("Namespace. Empty searches across namespaces."),
				},
			},
		},
		{
			Name:        ToolDeleteWorkload,
			Description: "Delete an AgentWorkload. Returns deleted=false (not an error) if the workload does not exist, so this is safe to call idempotently.",
			InputSchema: map[string]interface{}{
				"type":     "object",
				"required": []string{"name"},
				"properties": map[string]interface{}{
					"name":      stringProp("AgentWorkload metadata.name."),
					"namespace": stringProp("Namespace. Defaults to the operator's namespace."),
				},
			},
		},
	}
}
