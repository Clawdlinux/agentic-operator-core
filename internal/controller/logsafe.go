/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package controller

import (
	"strings"

	"github.com/Clawdlinux/agentic-operator-core/pkg/dataclass"
)

const maxLogTextBytes = 160

// safeLogText returns s for logs and status messages. Action names and the
// reasons built from them come from the MCP server, so they are untrusted. A
// value that carries a personal or credential data class is replaced by the
// class names, and everything is capped in length.
func safeLogText(s string) string {
	if classes := dataclass.Detect(s); len(classes) > 0 {
		return "[redacted:" + strings.Join(dataclass.Strings(classes), ",") + "]"
	}
	if len(s) > maxLogTextBytes {
		return s[:maxLogTextBytes] + "...[truncated]"
	}
	return s
}

func safeLogTexts(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = safeLogText(s)
	}
	return out
}
