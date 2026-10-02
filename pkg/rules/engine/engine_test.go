package engine

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Clawdlinux/agentic-operator-core/pkg/decision/input"
	"github.com/Clawdlinux/agentic-operator-core/pkg/rules/packs"
)

func src(name string, files map[string]string) Source {
	m := fstest.MapFS{}
	for p, body := range files {
		m[p] = &fstest.MapFile{Data: []byte(body)}
	}
	return Source{Name: name, FS: m}
}

const inlinePack = `package clawdlinux.decision

deny contains {"id": "T-01", "reason": "tool is rm"} if input.observed.tool == "rm"

require_approval contains {"id": "T-02", "reason": sprintf("tool %s", [input.observed.tool])} if {
	startswith(input.observed.tool, "scale")
}
`

func TestNewFailsClosed(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		src  Source
		want string
	}{
		{"nil fs", Source{Name: "x"}, "no files"},
		{"no modules", src("x", map[string]string{"README.md": "hi"}), "no rego modules"},
		{"only tests", src("x", map[string]string{"a_test.rego": "package clawdlinux.decision_test\n"}), "no rego modules"},
		{"parse error", src("x", map[string]string{"a.rego": "package clawdlinux.decision\ndeny contains"}), "parse"},
		{"wrong package", src("x", map[string]string{"a.rego": "package other\n"}), "declares package"},
		{"compile error", src("x", map[string]string{"a.rego": "package clawdlinux.decision\ndeny contains x if { y == 1 }\n"}), "compile"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New(ctx, tc.src)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("New err = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestEvalInline(t *testing.T) {
	ctx := context.Background()
	e, err := New(ctx, src("inline@v0.0.1", map[string]string{"p.rego": inlinePack}))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		tool        string
		wantDeny    []Finding
		wantApprove []Finding
	}{
		{"get", nil, nil},
		{"rm", []Finding{{"T-01", "tool is rm", "inline@v0.0.1"}}, nil},
		{"scale-up", nil, []Finding{{"T-02", "tool scale-up", "inline@v0.0.1"}}},
	}
	for _, tc := range tests {
		t.Run(tc.tool, func(t *testing.T) {
			v, err := e.Eval(ctx, Doc(input.Input{Observed: input.Observed{Tool: tc.tool}}))
			if err != nil {
				t.Fatal(err)
			}
			if len(v.Deny) == 0 {
				v.Deny = nil
			}
			if len(v.RequireApproval) == 0 {
				v.RequireApproval = nil
			}
			if !reflect.DeepEqual(v.Deny, tc.wantDeny) || !reflect.DeepEqual(v.RequireApproval, tc.wantApprove) {
				t.Fatalf("verdict = %+v", v)
			}
		})
	}
}

func TestEvalErrors(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name, rego string
	}{
		{"bad finding shape", `package clawdlinux.decision
deny contains "just a string" if true`},
		{"missing reason", `package clawdlinux.decision
deny contains {"id": "X"} if true`},
		{"deny not a set", `package clawdlinux.decision
deny := 1`},
		{"builtin error", `package clawdlinux.decision
deny contains {"id": "X", "reason": r} if r := sprintf("%d", [to_number("nope")])`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e, err := New(ctx, src("bad", map[string]string{"p.rego": tc.rego}))
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			if _, err := e.Eval(ctx, map[string]any{}); err == nil {
				t.Fatal("Eval must fail so the caller denies")
			}
		})
	}
}

func TestNilEngine(t *testing.T) {
	var e *Engine
	v, err := e.Eval(context.Background(), nil)
	if err != nil || len(v.Deny)+len(v.RequireApproval) != 0 {
		t.Fatalf("nil engine = %+v, %v", v, err)
	}
}

func TestLoadPacks(t *testing.T) {
	ctx := context.Background()
	if _, err := LoadPacks(ctx, "dpdp-in@v9.9.9"); err == nil {
		t.Fatal("unknown pack must fail")
	}
	e, err := LoadPacks(ctx, packs.Known()...)
	if err != nil {
		t.Fatal(err)
	}
	in := input.Input{
		Declared: input.Declared{Purpose: "kyc", DecisionType: "automated", AllowedDataClasses: []string{"aadhaar"}},
		Observed: input.Observed{DataClasses: []string{"aadhaar", "email"}},
	}
	v, err := e.Eval(ctx, Doc(in))
	if err != nil {
		t.Fatal(err)
	}
	var deny, approve []string
	for _, f := range v.Deny {
		deny = append(deny, f.Pack+"/"+f.RuleID)
	}
	for _, f := range v.RequireApproval {
		approve = append(approve, f.String())
	}
	if want := []string{"gdpr-eu@v0.1.0/GDPR-EU-02"}; !reflect.DeepEqual(deny, want) {
		t.Fatalf("deny = %v, want %v", deny, want)
	}
	if len(approve) != 2 || !strings.HasPrefix(approve[0], "dpdp-in@v0.1.0/DPDP-IN-04: ") || !strings.HasPrefix(approve[1], "gdpr-eu@v0.1.0/GDPR-EU-03: ") {
		t.Fatalf("approve = %v", approve)
	}
	again, err := LoadPacks(ctx, "gdpr-eu@v0.1.0")
	if err != nil || len(again.packs) != 1 {
		t.Fatalf("cached load = %v, %v", again, err)
	}
}

func TestDoc(t *testing.T) {
	c := 0.9
	d := Doc(input.Input{Claimed: input.AgentClaimed{Confidence: &c}})
	obs := d["observed"].(map[string]any)
	if got := obs["dataClasses"].([]any); got == nil || len(got) != 0 {
		t.Fatalf("dataClasses must be an empty list, got %#v", got)
	}
	if d["claimed"].(map[string]any)["confidence"] != 0.9 {
		t.Fatalf("claimed = %#v", d["claimed"])
	}
	if _, ok := d["claimed"].(map[string]any)["clusterHealth"]; ok {
		t.Fatal("absent clusterHealth must be omitted")
	}
}
