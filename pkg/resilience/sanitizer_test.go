package resilience

import (
	"strings"
	"testing"
)

func TestSanitizer_ScrubsBearerTokens(t *testing.T) {
	s := NewSanitizer()
	input := "Authorization: Bearer sk-1234567890abcdefghij"
	result := s.Scrub(input)
	if strings.Contains(result, "sk-1234567890abcdefghij") {
		t.Errorf("bearer token not scrubbed: %s", result)
	}
	if !strings.Contains(result, "REDACTED") {
		t.Errorf("expected REDACTED in output: %s", result)
	}
}

func TestSanitizer_ScrubsJWT(t *testing.T) {
	s := NewSanitizer()
	input := "token: eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.signature_here"
	result := s.Scrub(input)
	if strings.Contains(result, "eyJhbGci") {
		t.Errorf("JWT not scrubbed: %s", result)
	}
}

func TestSanitizer_ScrubsAPIKeys(t *testing.T) {
	s := NewSanitizer()
	input := `api_key: "sk-abcdefghijklmnop1234"`
	result := s.Scrub(input)
	if strings.Contains(result, "sk-abcdefghijklmnop1234") {
		t.Errorf("API key not scrubbed: %s", result)
	}
}

func TestSanitizer_ScrubsEmails(t *testing.T) {
	s := NewSanitizer()
	input := "User email: john.doe@company.com contacted support"
	result := s.Scrub(input)
	if strings.Contains(result, "john.doe@company.com") {
		t.Errorf("email not scrubbed: %s", result)
	}
}

func TestSanitizer_PreservesCleanText(t *testing.T) {
	s := NewSanitizer()
	input := "Agent completed task with quality score 85"
	result := s.Scrub(input)
	if result != input {
		t.Errorf("clean text modified: %q → %q", input, result)
	}
}

func TestSanitizer_ScrubMap(t *testing.T) {
	s := NewSanitizer()
	m := map[string]string{
		"name":   "test-agent",
		"secret": "api_key: sk-abcdefghijklmnopqrst",
	}
	result := s.ScrubMap(m)
	if result["name"] != "test-agent" {
		t.Errorf("name changed unexpectedly: %s", result["name"])
	}
	if strings.Contains(result["secret"], "sk-abcdefghijklmnopqrst") {
		t.Errorf("secret not scrubbed in map: %s", result["secret"])
	}
}
