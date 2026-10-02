package invariants

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Clawdlinux/agentic-operator-core/pkg/decision/input"
)

func ids(rs []Result) []string {
	var out []string
	for _, r := range rs {
		out = append(out, r.ID)
	}
	return out
}

func TestCheck(t *testing.T) {
	declared := input.Declared{
		Purpose:             "refunds",
		AllowedDataClasses:  []string{"email"},
		AllowedDestinations: []string{"mcp.internal"},
	}
	tests := []struct {
		name string
		in   input.Input
		ctx  Context
		want []string
	}{
		{"empty input passes", input.Input{}, Context{}, nil},
		{"undeclared ignores destination and classes",
			input.Input{Observed: input.Observed{Destination: "evil.example", DataClasses: []string{"card", "email"}}},
			Context{}, nil},

		{"INV-01 fail without declared intent",
			input.Input{Observed: input.Observed{DataClasses: []string{"credential"}}},
			Context{}, []string{"INV-01"}},
		{"INV-01 fail even when declared allows credential",
			input.Input{
				Declared: input.Declared{AllowedDataClasses: []string{"credential"}, AllowedDestinations: []string{"mcp.internal"}},
				Observed: input.Observed{Destination: "mcp.internal", DataClasses: []string{"credential"}},
			},
			Context{}, []string{"INV-01"}},

		{"INV-02 pass declared destination",
			input.Input{Declared: declared, Observed: input.Observed{Destination: "mcp.internal"}},
			Context{}, nil},
		{"INV-02 fail undeclared destination",
			input.Input{Declared: declared, Observed: input.Observed{Destination: "evil.example"}},
			Context{}, []string{"INV-02"}},
		{"INV-02 fail empty allow list once declared",
			input.Input{Declared: input.Declared{Purpose: "x"}, Observed: input.Observed{Destination: "mcp.internal"}},
			Context{}, []string{"INV-02"}},

		{"INV-03 pass personal data to declared destination",
			input.Input{Declared: declared, Observed: input.Observed{Destination: "mcp.internal", DataClasses: []string{"email"}}},
			Context{}, nil},
		{"INV-03 fail personal data to undeclared destination",
			input.Input{Declared: declared, Observed: input.Observed{Destination: "evil.example", DataClasses: []string{"email"}}},
			Context{}, []string{"INV-02", "INV-03"}},

		{"INV-04 pass subset",
			input.Input{Declared: declared, Observed: input.Observed{Destination: "mcp.internal", DataClasses: []string{"EMAIL"}}},
			Context{}, nil},
		{"INV-04 fail class not declared",
			input.Input{Declared: declared, Observed: input.Observed{Destination: "mcp.internal", DataClasses: []string{"card", "email"}}},
			Context{}, []string{"INV-04"}},

		{"INV-05 pass receipts not required", input.Input{}, Context{ReceiptsRequired: false}, nil},
		{"INV-05 pass writer available", input.Input{}, Context{ReceiptsRequired: true, ReceiptWriterAvailable: true}, nil},
		{"INV-05 fail writer missing", input.Input{}, Context{ReceiptsRequired: true}, []string{"INV-05"}},

		{"INV-06 pass scan complete", input.Input{}, Context{}, nil},
		{"INV-06 fail scan incomplete", input.Input{Observed: input.Observed{ScanIncomplete: true}}, Context{}, []string{"INV-06"}},

		{"all fire in order",
			input.Input{Declared: declared, Observed: input.Observed{Destination: "evil.example", DataClasses: []string{"credential", "pan"}}},
			Context{ReceiptsRequired: true}, []string{"INV-01", "INV-02", "INV-03", "INV-04", "INV-05"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ids(Check(tc.in, tc.ctx)); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Check IDs = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestReasonsHaveNoContent(t *testing.T) {
	in := input.Input{
		Declared: input.Declared{Purpose: "x"},
		Observed: input.Observed{Destination: "evil.example", DataClasses: []string{"credential", "email"}},
	}
	for _, r := range Check(in, Context{}) {
		if r.Reason == "" || strings.Contains(r.Reason, "AKIA") {
			t.Fatalf("bad reason %q", r.Reason)
		}
		if !strings.HasPrefix(r.String(), r.ID+": ") {
			t.Fatalf("String = %q", r.String())
		}
	}
}

func TestIDsStableAndBounded(t *testing.T) {
	want := []string{"INV-01", "INV-02", "INV-03", "INV-04", "INV-05", "INV-06"}
	if got := IDs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("IDs = %v, want %v", got, want)
	}
	if len(IDs()) >= 15 {
		t.Fatal("invariant set must stay under 15")
	}
}
