package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNewRootCommand_ContainsRequiredSubcommands(t *testing.T) {
	cmd := newRootCommand()

	required := []string{"get", "describe", "logs", "cost", "apply", "version"}
	for _, name := range required {
		if child, _, err := cmd.Find([]string{name}); err != nil || child == nil || child.Name() != name {
			t.Fatalf("expected subcommand %q to exist", name)
		}
	}
}

func TestCLIOptionsValidateOutput(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "default empty to table", input: "", want: "table"},
		{name: "table", input: "table", want: "table"},
		{name: "json", input: "json", want: "json"},
		{name: "yaml", input: "yaml", want: "yaml"},
		{name: "mixed case", input: "YaMl", want: "yaml"},
		{name: "invalid", input: "xml", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := &cliOptions{Output: tc.input}
			err := opts.validateOutput()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for output %q", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateOutput returned error: %v", err)
			}
			if opts.Output != tc.want {
				t.Fatalf("output = %q, want %q", opts.Output, tc.want)
			}
		})
	}
}

func TestDecodeYAMLDocuments_MultiDoc(t *testing.T) {
	content := []byte(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-one
---
apiVersion: agentic.clawdlinux.org/v1alpha1
kind: AgentWorkload
metadata:
  name: aw-one
spec:
  agents:
    - planner
`)

	docs, err := decodeYAMLDocuments(content)
	if err != nil {
		t.Fatalf("decodeYAMLDocuments returned error: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("decoded docs = %d, want 2", len(docs))
	}
	if docs[0].GetKind() != "ConfigMap" {
		t.Fatalf("first kind = %q, want ConfigMap", docs[0].GetKind())
	}
	if docs[1].GetKind() != "AgentWorkload" {
		t.Fatalf("second kind = %q, want AgentWorkload", docs[1].GetKind())
	}
}

func TestValidateAgentWorkloadManifest(t *testing.T) {
	validDocs, err := decodeYAMLDocuments([]byte(`
apiVersion: agentic.clawdlinux.org/v1alpha1
kind: AgentWorkload
metadata:
  name: sample
spec:
  agents:
    - analyst
  mcpServerEndpoint: https://example.com
`))
	if err != nil {
		t.Fatalf("decode valid manifest: %v", err)
	}
	if err := validateAgentWorkloadManifest(validDocs[0]); err != nil {
		t.Fatalf("expected valid manifest, got error: %v", err)
	}

	invalidDocs, err := decodeYAMLDocuments([]byte(`
apiVersion: agentic.clawdlinux.org/v1alpha1
kind: AgentWorkload
metadata:
  name: broken
spec:
  mcpServerEndpoint: http://insecure.example.com
`))
	if err != nil {
		t.Fatalf("decode invalid manifest: %v", err)
	}
	if err := validateAgentWorkloadManifest(invalidDocs[0]); err == nil {
		t.Fatal("expected validation error for missing agents and insecure endpoint")
	}
}

func TestPrintStructured_JSONAndYAML(t *testing.T) {
	rows := []workloadRow{{Name: "w1", Namespace: "ns1", Status: "Running", Model: "m1", CostToday: 1.23, Age: "1h"}}

	var jsonBuf bytes.Buffer
	if err := printStructured(&jsonBuf, rows, "json"); err != nil {
		t.Fatalf("printStructured json error: %v", err)
	}
	var decoded []workloadRow
	if err := json.Unmarshal(jsonBuf.Bytes(), &decoded); err != nil {
		t.Fatalf("json output is invalid: %v", err)
	}
	if len(decoded) != 1 || decoded[0].Name != "w1" {
		t.Fatalf("unexpected json decoded output: %+v", decoded)
	}

	var yamlBuf bytes.Buffer
	if err := printStructured(&yamlBuf, rows, "yaml"); err != nil {
		t.Fatalf("printStructured yaml error: %v", err)
	}
	if !strings.Contains(yamlBuf.String(), "name: w1") {
		t.Fatalf("yaml output missing expected field, got: %s", yamlBuf.String())
	}
}

func TestExtractRecords(t *testing.T) {
	record := map[string]interface{}{"workload": "alpha", "cost": 0.5}

	tests := []struct {
		name    string
		payload interface{}
		wantLen int
	}{
		{name: "array payload", payload: []interface{}{record}, wantLen: 1},
		{name: "map data payload", payload: map[string]interface{}{"data": []interface{}{record}}, wantLen: 1},
		{name: "map logs payload", payload: map[string]interface{}{"logs": []interface{}{record}}, wantLen: 1},
		{name: "empty payload", payload: map[string]interface{}{}, wantLen: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractRecords(tc.payload)
			if len(got) != tc.wantLen {
				t.Fatalf("records len = %d, want %d", len(got), tc.wantLen)
			}
		})
	}
}
