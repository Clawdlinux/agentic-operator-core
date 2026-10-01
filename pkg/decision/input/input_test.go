package input

import (
	"reflect"
	"testing"
)

func TestViolations(t *testing.T) {
	declared := Declared{
		Purpose:             "refund triage",
		AllowedDataClasses:  []string{"email"},
		AllowedDestinations: []string{"mcp.internal", "*.corp.example.com"},
	}
	tests := []struct {
		name string
		in   Input
		want []string
	}{
		{"undeclared returns nil", Input{Observed: Observed{Destination: "evil.io", DataClasses: []string{"card"}}}, nil},
		{"all within", Input{Declared: declared, Observed: Observed{Destination: "https://mcp.internal:8000", DataClasses: []string{"email"}}}, nil},
		{"wildcard subdomain", Input{Declared: declared, Observed: Observed{Destination: "a.b.corp.example.com"}}, nil},
		{"wildcard does not match apex", Input{Declared: declared, Observed: Observed{Destination: "corp.example.com"}},
			[]string{`observed destination "corp.example.com" not declared`}},
		{"suffix trick rejected", Input{Declared: declared, Observed: Observed{Destination: "evilcorp.example.com"}},
			[]string{`observed destination "evilcorp.example.com" not declared`}},
		{"case insensitive host", Input{Declared: declared, Observed: Observed{Destination: "HTTPS://MCP.Internal"}}, nil},
		{"undeclared class", Input{Declared: declared, Observed: Observed{Destination: "mcp.internal", DataClasses: []string{"pan", "card", "email"}}},
			[]string{"observed data classes not declared: card,pan"}},
		{"both", Input{Declared: declared, Observed: Observed{Destination: "x.io:443", DataClasses: []string{"iban"}}},
			[]string{"observed data classes not declared: iban", `observed destination "x.io" not declared`}},
		{"declared empty lists allow nothing", Input{Declared: Declared{Purpose: "p"}, Observed: Observed{Destination: "mcp.internal", DataClasses: []string{"email"}}},
			[]string{"observed data classes not declared: email", `observed destination "mcp.internal" not declared`}},
		{"no destination observed", Input{Declared: Declared{Purpose: "p"}}, nil},
		{"claims ignored", Input{Declared: declared, Observed: Observed{Destination: "x.io"}, Claimed: AgentClaimed{Intent: "trust me"}},
			[]string{`observed destination "x.io" not declared`}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.in.Violations(); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Violations = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestHost(t *testing.T) {
	tests := map[string]string{
		"":                          "",
		"https://mcp-server:8000":   "mcp-server",
		"http://127.0.0.1:9000/x":   "127.0.0.1",
		"api.example.com:443":       "api.example.com",
		"API.Example.com":           "api.example.com",
		"https://[::1]:8443/path":   "::1",
		"  https://h.example.com  ": "h.example.com",
	}
	for in, want := range tests {
		if got := Host(in); got != want {
			t.Errorf("Host(%q) = %q, want %q", in, got, want)
		}
	}
}
