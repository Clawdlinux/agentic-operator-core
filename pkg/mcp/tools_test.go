package mcp

import "testing"

func TestRegisteredToolsIncludesDemoCriticalTools(t *testing.T) {
	t.Parallel()

	tools := registeredTools()
	seen := map[string]bool{}
	for _, tool := range tools {
		seen[string(tool.Name)] = true
	}
	for _, name := range []ToolName{ToolCreateWorkload, ToolGetWorkloadStatus, ToolListWorkloads, ToolGetWorkloadLogs, ToolGetWorkloadCost, ToolDeleteWorkload} {
		if !seen[string(name)] {
			t.Fatalf("tool %q missing", name)
		}
	}
}

func TestRegisteredToolsRejectDuplicateNames(t *testing.T) {
	t.Parallel()

	seen := map[string]bool{}
	for _, tool := range registeredTools() {
		name := string(tool.Name)
		if seen[name] {
			t.Fatalf("duplicate tool name %q", tool.Name)
		}
		seen[name] = true
	}
}

func TestCreateWorkloadToolSchemaRequiresSafetyInputs(t *testing.T) {
	t.Parallel()

	var createTool *ToolDescriptor
	for _, tool := range registeredTools() {
		if tool.Name == ToolCreateWorkload {
			tool := tool
			createTool = &tool
			break
		}
	}
	if createTool == nil {
		t.Fatal("create_workload tool missing")
	}
	required, ok := createTool.InputSchema["required"].([]string)
	if !ok {
		t.Fatalf("required schema = %#v, want []string", createTool.InputSchema["required"])
	}
	for _, want := range []string{"name", "objective", "agents"} {
		if !containsString(required, want) {
			t.Fatalf("required fields = %v, want %q", required, want)
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
