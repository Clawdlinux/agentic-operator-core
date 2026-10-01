package packs_test

import (
	"context"
	"regexp"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/tester"

	"github.com/Clawdlinux/agentic-operator-core/pkg/rules/engine"
	"github.com/Clawdlinux/agentic-operator-core/pkg/rules/packs"
)

func TestKnownMatchesCRDPattern(t *testing.T) {
	want := []string{"dpdp-in@v0.1.0", "gdpr-eu@v0.1.0"}
	got := packs.Known()
	if len(got) != len(want) {
		t.Fatalf("Known = %v, want %v", got, want)
	}
	re := regexp.MustCompile(packs.NamePattern)
	for i, n := range got {
		if n != want[i] || !re.MatchString(n) || !packs.Valid(n) {
			t.Fatalf("pack %q invalid", n)
		}
	}
	for _, bad := range []string{"", "dpdp-in", "dpdp-in@v0.2.0", "hipaa@v0.1.0", "../x"} {
		if packs.Valid(bad) {
			t.Fatalf("%q must be invalid", bad)
		}
		if _, err := packs.FS(bad); err == nil {
			t.Fatalf("FS(%q) must fail", bad)
		}
	}
}

// TestRegoUnitTests runs every pack's _test.rego files, like `opa test`.
func TestRegoUnitTests(t *testing.T) {
	for _, name := range packs.Known() {
		t.Run(name, func(t *testing.T) {
			fsys, err := packs.FS(name)
			if err != nil {
				t.Fatal(err)
			}
			mods := map[string]*ast.Module{}
			for _, tests := range []bool{false, true} {
				files, err := engine.Modules(fsys, tests)
				if err != nil {
					t.Fatal(err)
				}
				for p, body := range files {
					m, err := ast.ParseModule(p, body)
					if err != nil {
						t.Fatalf("parse %s: %v", p, err)
					}
					mods[p] = m
				}
			}
			ch, err := tester.NewRunner().SetModules(mods).RunTests(context.Background(), nil)
			if err != nil {
				t.Fatal(err)
			}
			n := 0
			for r := range ch {
				n++
				if r.Error != nil || r.Fail {
					t.Errorf("%s: %s", r.Name, r.String())
				}
			}
			if n == 0 {
				t.Fatal("no rego tests found")
			}
			t.Logf("%d rego tests", n)
		})
	}
}
