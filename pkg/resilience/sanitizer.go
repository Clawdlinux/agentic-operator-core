package resilience

import (
	"regexp"
	"strings"
)

// Sanitizer scrubs sensitive data from log messages before they're persisted.
type Sanitizer struct {
	patterns []*regexp.Regexp
}

// NewSanitizer returns a Sanitizer pre-loaded with common PII/secret patterns.
func NewSanitizer() *Sanitizer {
	return &Sanitizer{
		patterns: []*regexp.Regexp{
			// API keys / bearer tokens
			regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9_\-\.]{20,}`),
			regexp.MustCompile(`(?i)(api[_-]?key|api[_-]?token|secret|password|credential)["\s:=]+[A-Za-z0-9_\-\.]{8,}`),
			// JWT tokens (three base64 segments separated by dots)
			regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`),
			// Email addresses
			regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`),
			// Credit card numbers (basic)
			regexp.MustCompile(`\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`),
			// AWS keys
			regexp.MustCompile(`AKIA[A-Z0-9]{16}`),
			// Cloudflare API tokens
			regexp.MustCompile(`[A-Za-z0-9_-]{35,45}`),
		},
	}
}

// Scrub replaces sensitive data with [REDACTED].
func (s *Sanitizer) Scrub(input string) string {
	result := input
	for _, pattern := range s.patterns {
		result = pattern.ReplaceAllStringFunc(result, func(match string) string {
			// Keep prefix for context, redact the value
			if idx := strings.IndexAny(match, ":= "); idx > 0 && idx < len(match)-1 {
				return match[:idx+1] + "[REDACTED]"
			}
			if len(match) > 8 {
				return match[:4] + "[REDACTED]"
			}
			return "[REDACTED]"
		})
	}
	return result
}

// ScrubMap sanitizes all string values in a map (useful for log fields).
func (s *Sanitizer) ScrubMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = s.Scrub(v)
	}
	return out
}
