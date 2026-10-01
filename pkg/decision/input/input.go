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
	if v := in.DataClassViolation(); v != "" {
		out = append(out, v)
	}
	if v := in.DestinationViolation(); v != "" {
		out = append(out, v)
	}
	return out
}

// UndeclaredDataClasses returns observed classes missing from the declared
// allow list, sorted. It returns nil when nothing is declared.
func (in Input) UndeclaredDataClasses() []string {
	if in.Declared.IsZero() {
		return nil
	}
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
	sort.Strings(undeclared)
	return undeclared
}

// DataClassViolation returns a reason when observed data classes fall outside
// the declared allow list, or "" when they do not or nothing is declared.
func (in Input) DataClassViolation() string {
	if u := in.UndeclaredDataClasses(); len(u) > 0 {
		return fmt.Sprintf("observed data classes not declared: %s", strings.Join(u, ","))
	}
	return ""
}

// DestinationDeclared reports whether the observed destination is in the
// declared allow list. An empty destination counts as declared.
func (in Input) DestinationDeclared() bool {
	host := Host(in.Observed.Destination)
	return host == "" || hostAllowed(host, in.Declared.AllowedDestinations)
}

// DestinationViolation returns a reason when the observed destination is not
// declared, or "" when it is or nothing is declared.
func (in Input) DestinationViolation() string {
	if in.Declared.IsZero() || in.DestinationDeclared() {
		return ""
	}
	return fmt.Sprintf("observed destination %q not declared", Host(in.Observed.Destination))
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
