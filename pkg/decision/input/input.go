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

// Package input holds decision inputs split by trust: declared by a human in
// the manifest, observed by the platform, and claimed by the agent. See
// docs/policy-input.md.
package input

import (
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
)

// Declared comes from spec.declaredIntent and is reviewed in a PR.
type Declared struct {
	Purpose             string
	DecisionType        string
	AllowedDataClasses  []string
	AllowedDestinations []string
}

// Observed is measured by the platform, not reported by the agent.
type Observed struct {
	Destination      string
	DataClasses      []string
	Tool             string
	CallerIdentity   string
	PriorActionCount int
	PriorDeniedCount int
}

// AgentClaimed is self-reported by the agent. It may only tighten a decision.
type AgentClaimed struct {
	Confidence    *float64
	ClusterHealth *float64
	Intent        string
}

// Input is the full decision input.
type Input struct {
	Declared Declared
	Observed Observed
	Claimed  AgentClaimed
}

// IsZero reports whether nothing was declared.
func (d Declared) IsZero() bool {
	return d.Purpose == "" && d.DecisionType == "" &&
		len(d.AllowedDataClasses) == 0 && len(d.AllowedDestinations) == 0
}

// Violations returns reasons the observed facts fall outside the declared
// intent. It returns nil when nothing is declared; callers treat that as
// undeclared. Once anything is declared, an empty allow list allows nothing.
func (in Input) Violations() []string {
	if in.Declared.IsZero() {
		return nil
	}
	var out []string

	allowed := map[string]bool{}
	for _, c := range in.Declared.AllowedDataClasses {
		allowed[strings.ToLower(strings.TrimSpace(c))] = true
	}
	var undeclared []string
	for _, c := range in.Observed.DataClasses {
		if !allowed[strings.ToLower(c)] {
			undeclared = append(undeclared, c)
		}
	}
	if len(undeclared) > 0 {
		sort.Strings(undeclared)
		out = append(out, fmt.Sprintf("observed data classes not declared: %s", strings.Join(undeclared, ",")))
	}

	if host := Host(in.Observed.Destination); host != "" && !hostAllowed(host, in.Declared.AllowedDestinations) {
		out = append(out, fmt.Sprintf("observed destination %q not declared", host))
	}
	return out
}

// Host extracts the lower-case host from a URL or host[:port] string.
func Host(dest string) string {
	dest = strings.TrimSpace(dest)
	if dest == "" {
		return ""
	}
	if strings.Contains(dest, "://") {
		u, err := url.Parse(dest)
		if err != nil {
			return ""
		}
		return strings.ToLower(u.Hostname())
	}
	if h, _, err := net.SplitHostPort(dest); err == nil {
		return strings.ToLower(h)
	}
	return strings.ToLower(strings.Trim(dest, "[]"))
}

func hostAllowed(host string, patterns []string) bool {
	for _, p := range patterns {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "*.") {
			if suffix := p[1:]; strings.HasSuffix(host, suffix) && len(host) > len(suffix) {
				return true
			}
			continue
		}
		if host == p {
			return true
		}
	}
	return false
}
