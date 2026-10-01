package observe

import (
	"reflect"
	"testing"

	agenticv1alpha1 "github.com/Clawdlinux/agentic-operator-core/api/v1alpha1"
	"github.com/Clawdlinux/agentic-operator-core/pkg/decision/input"
)

func TestObserve(t *testing.T) {
	tests := []struct {
		name string
		in   Facts
		want input.Observed
	}{
		{"empty", Facts{}, input.Observed{DataClasses: []string{}}},
		{
			"full",
			Facts{
				Endpoint:         "https://mcp-server:8000",
				Tool:             "refund",
				CallerIdentity:   "system:serviceaccount:ops:bot",
				Payload:          map[string]any{"action": "refund", "description": "refund a@b.io card 4111111111111111"},
				PriorActionCount: 3,
				PriorDeniedCount: 1,
			},
			input.Observed{
				Destination:      "mcp-server",
				DataClasses:      []string{"card", "email"},
				Tool:             "refund",
				CallerIdentity:   "system:serviceaccount:ops:bot",
				PriorActionCount: 3,
				PriorDeniedCount: 1,
			},
		},
		{"bad endpoint", Facts{Endpoint: "://"}, input.Observed{DataClasses: []string{}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Observe(tc.in); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Observe = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestHistory(t *testing.T) {
	yes, no := true, false
	tests := []struct {
		name             string
		executed         []agenticv1alpha1.Action
		proposed         []agenticv1alpha1.Action
		wantTotal, wantD int
	}{
		{"none", nil, nil, 0, 0},
		{"mixed", []agenticv1alpha1.Action{{Name: "a"}}, []agenticv1alpha1.Action{{Approved: &no}, {Approved: &yes}, {}}, 4, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			total, denied := History(tc.executed, tc.proposed)
			if total != tc.wantTotal || denied != tc.wantD {
				t.Fatalf("History = %d,%d want %d,%d", total, denied, tc.wantTotal, tc.wantD)
			}
		})
	}
}

func TestDeclared(t *testing.T) {
	if got := Declared(nil); !got.IsZero() {
		t.Fatalf("nil intent should be zero, got %#v", got)
	}
	d := &agenticv1alpha1.DeclaredIntent{
		Purpose:             "p",
		DecisionType:        "refund",
		AllowedDataClasses:  []string{"email"},
		AllowedDestinations: []string{"mcp.internal"},
	}
	want := input.Declared{Purpose: "p", DecisionType: "refund", AllowedDataClasses: []string{"email"}, AllowedDestinations: []string{"mcp.internal"}}
	if got := Declared(d); !reflect.DeepEqual(got, want) {
		t.Fatalf("Declared = %#v, want %#v", got, want)
	}
}
