/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package controller

import (
	"strings"
	"testing"
)

func TestSafeLogText(t *testing.T) {
	if got := safeLogText("restart_pod"); got != "restart_pod" {
		t.Fatalf("plain name changed: %q", got)
	}
	for _, in := range []string{"use key AKIAIOSFODNN7EXAMPLE", "mail canary.person@example.com now"} {
		got := safeLogText(in)
		if strings.Contains(got, "AKIA") || strings.Contains(got, "@") || !strings.HasPrefix(got, "[redacted:") {
			t.Fatalf("sensitive text not redacted: %q", got)
		}
	}
	long := strings.Repeat("a", 1000)
	if got := safeLogText(long); len(got) > maxLogTextBytes+20 {
		t.Fatalf("long text not capped: %d bytes", len(got))
	}
	out := safeLogTexts([]string{"ok", "AKIAIOSFODNN7EXAMPLE"})
	if out[0] != "ok" || strings.Contains(out[1], "AKIA") {
		t.Fatalf("safeLogTexts = %v", out)
	}
}
