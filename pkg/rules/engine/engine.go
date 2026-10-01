/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package engine evaluates Rego policy packs with the embedded OPA Go
// library. Each pack compiles once. A compile or eval error is returned to the
// caller, which must treat it as deny. Packs can only add deny or
// require_approval findings. See docs/policy-packs.md.
package engine

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/rego"

	"github.com/Clawdlinux/agentic-operator-core/pkg/decision/input"
	"github.com/Clawdlinux/agentic-operator-core/pkg/rules/packs"
)

// Package is the Rego package every pack module must declare.
const Package = "clawdlinux.decision"

const query = "data." + Package

// Finding is one rule result from a pack.
type Finding struct {
	RuleID string
	Reason string
	Pack   string
}

// String formats a finding as "pack/RULE: reason".
func (f Finding) String() string { return f.Pack + "/" + f.RuleID + ": " + f.Reason }

// Verdict collects findings from every loaded pack, sorted.
type Verdict struct {
	Deny            []Finding
	RequireApproval []Finding
}

// Source is one pack: a name for findings and the files holding its Rego.
type Source struct {
	Name string
	FS   fs.FS
}

type compiled struct {
	name  string
	query rego.PreparedEvalQuery
}

// Engine holds compiled packs. It is safe for concurrent use.
type Engine struct {
	packs []compiled
}

// New compiles each source once. Any read, parse, or compile error fails.
func New(ctx context.Context, srcs ...Source) (*Engine, error) {
	e := &Engine{}
	for _, s := range srcs {
		c, err := compile(ctx, s)
		if err != nil {
			return nil, err
		}
		e.packs = append(e.packs, c)
	}
	return e, nil
}

// Modules reads the non-test .rego files in fsys, keyed by path.
func Modules(fsys fs.FS, tests bool) (map[string]string, error) {
	out := map[string]string{}
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || path.Ext(p) != ".rego" {
			return nil
		}
		if strings.HasSuffix(p, "_test.rego") != tests {
			return nil
		}
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		out[p] = string(b)
		return nil
	})
	return out, err
}

func compile(ctx context.Context, s Source) (compiled, error) {
	if s.FS == nil {
		return compiled{}, fmt.Errorf("engine: pack %q has no files", s.Name)
	}
	mods, err := Modules(s.FS, false)
	if err != nil {
		return compiled{}, fmt.Errorf("engine: read pack %q: %w", s.Name, err)
	}
	if len(mods) == 0 {
		return compiled{}, fmt.Errorf("engine: pack %q has no rego modules", s.Name)
	}
	opts := []func(*rego.Rego){rego.Query(query), rego.StrictBuiltinErrors(true)}
	for _, p := range sortedKeys(mods) {
		m, err := ast.ParseModule(p, mods[p])
		if err != nil {
			return compiled{}, fmt.Errorf("engine: parse %s/%s: %w", s.Name, p, err)
		}
		if got := strings.TrimPrefix(m.Package.Path.String(), "data."); got != Package {
			return compiled{}, fmt.Errorf("engine: %s/%s declares package %q, want %q", s.Name, p, got, Package)
		}
		opts = append(opts, rego.ParsedModule(m))
	}
	pq, err := rego.New(opts...).PrepareForEval(ctx)
	if err != nil {
		return compiled{}, fmt.Errorf("engine: compile pack %q: %w", s.Name, err)
	}
	return compiled{name: s.Name, query: pq}, nil
}

// Eval runs every pack against in. Any error means the caller must deny.
func (e *Engine) Eval(ctx context.Context, in any) (Verdict, error) {
	var v Verdict
	if e == nil {
		return v, nil
	}
	for _, c := range e.packs {
		rs, err := c.query.Eval(ctx, rego.EvalInput(in))
		if err != nil {
			return Verdict{}, fmt.Errorf("engine: eval pack %q: %w", c.name, err)
		}
		if len(rs) != 1 || len(rs[0].Expressions) != 1 {
			return Verdict{}, fmt.Errorf("engine: pack %q returned %d results", c.name, len(rs))
		}
		doc, ok := rs[0].Expressions[0].Value.(map[string]any)
		if !ok {
			return Verdict{}, fmt.Errorf("engine: pack %q returned %T", c.name, rs[0].Expressions[0].Value)
		}
		deny, err := findings(c.name, "deny", doc["deny"])
		if err != nil {
			return Verdict{}, err
		}
		approve, err := findings(c.name, "require_approval", doc["require_approval"])
		if err != nil {
			return Verdict{}, err
		}
		v.Deny = append(v.Deny, deny...)
		v.RequireApproval = append(v.RequireApproval, approve...)
	}
	sortFindings(v.Deny)
	sortFindings(v.RequireApproval)
	return v, nil
}

func findings(pack, rule string, raw any) ([]Finding, error) {
	if raw == nil {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("engine: pack %q %s is %T, want a set", pack, rule, raw)
	}
	out := make([]Finding, 0, len(items))
	for _, it := range items {
		obj, ok := it.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("engine: pack %q %s item is %T, want object", pack, rule, it)
		}
		id, _ := obj["id"].(string)
		reason, _ := obj["reason"].(string)
		if id == "" || reason == "" {
			return nil, fmt.Errorf("engine: pack %q %s item needs string id and reason", pack, rule)
		}
		out = append(out, Finding{RuleID: id, Reason: reason, Pack: pack})
	}
	return out, nil
}

// Doc converts a decision input into the Rego input document. Field names
// match docs/policy-packs.md.
func Doc(in input.Input) map[string]any {
	doc := map[string]any{
		"declared": map[string]any{
			"purpose":             in.Declared.Purpose,
			"decisionType":        in.Declared.DecisionType,
			"allowedDataClasses":  list(in.Declared.AllowedDataClasses),
			"allowedDestinations": list(in.Declared.AllowedDestinations),
		},
		"observed": map[string]any{
			"destination":      in.Observed.Destination,
			"dataClasses":      list(in.Observed.DataClasses),
			"tool":             in.Observed.Tool,
			"callerIdentity":   in.Observed.CallerIdentity,
			"priorActionCount": in.Observed.PriorActionCount,
			"priorDeniedCount": in.Observed.PriorDeniedCount,
		},
	}
	claimed := map[string]any{"intent": in.Claimed.Intent}
	if in.Claimed.Confidence != nil {
		claimed["confidence"] = *in.Claimed.Confidence
	}
	if in.Claimed.ClusterHealth != nil {
		claimed["clusterHealth"] = *in.Claimed.ClusterHealth
	}
	doc["claimed"] = claimed
	return doc
}

var (
	cacheMu sync.Mutex
	cache   = map[string]compiled{}
)

// LoadPacks returns an engine for shipped packs by name. Each pack compiles
// once per process. Unknown names and compile errors fail.
func LoadPacks(ctx context.Context, names ...string) (*Engine, error) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	e := &Engine{}
	for _, n := range names {
		c, ok := cache[n]
		if !ok {
			fsys, err := packs.FS(n)
			if err != nil {
				return nil, err
			}
			if c, err = compile(ctx, Source{Name: n, FS: fsys}); err != nil {
				return nil, err
			}
			cache[n] = c
		}
		e.packs = append(e.packs, c)
	}
	return e, nil
}

func list(s []string) []any {
	out := make([]any, len(s))
	for i, v := range s {
		out[i] = v
	}
	return out
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortFindings(xs []Finding) {
	sort.Slice(xs, func(i, j int) bool {
		if xs[i].Pack != xs[j].Pack {
			return xs[i].Pack < xs[j].Pack
		}
		if xs[i].RuleID != xs[j].RuleID {
			return xs[i].RuleID < xs[j].RuleID
		}
		return xs[i].Reason < xs[j].Reason
	})
}
