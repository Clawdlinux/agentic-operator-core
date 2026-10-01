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

// Package invariants holds the deterministic floor of the decision
// architecture. Each invariant has a stable ID and denies on its own. No
// policy pack, threshold, or agent claim can override an invariant result.
// Pure Go: no model, no network. See docs/architecture/invariants.md.
package invariants

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Clawdlinux/agentic-operator-core/pkg/dataclass"
	"github.com/Clawdlinux/agentic-operator-core/pkg/decision/input"
)

// Stable invariant IDs. Never reuse or renumber an ID.
const (
	CredentialsNeverReachAgent  = "INV-01"
	EgressOnlyDeclared          = "INV-02"
	PersonalDataDeclaredDest    = "INV-03"
	DataClassesSubsetOfDeclared = "INV-04"
	ReceiptsOrFailClosed        = "INV-05"
)

// Result is one invariant violation. Reason never contains matched content.
type Result struct {
	ID     string
	Reason string
}

// String formats a result as "ID: reason".
func (r Result) String() string { return r.ID + ": " + r.Reason }

// Context carries platform facts that are not part of the decision input.
type Context struct {
	// ReceiptsRequired is true when every decision must produce a receipt.
	ReceiptsRequired bool
	// ReceiptWriterAvailable is true when a receipt writer is ready.
	ReceiptWriterAvailable bool
}

type invariant struct {
	id    string
	check func(input.Input, Context) string
}

// all is the full, ordered invariant set. Keep it under 15 entries.
var all = []invariant{
	{CredentialsNeverReachAgent, credentials},
	{EgressOnlyDeclared, egress},
	{PersonalDataDeclaredDest, personalDest},
	{DataClassesSubsetOfDeclared, dataClassSubset},
	{ReceiptsOrFailClosed, receipts},
}

// IDs returns every invariant ID in check order.
func IDs() []string {
	out := make([]string, len(all))
	for i, inv := range all {
		out[i] = inv.id
	}
	return out
}

// Check runs every invariant and returns the violations in ID order. An empty
// result means no invariant denies the action.
func Check(in input.Input, ctx Context) []Result {
	var out []Result
	for _, inv := range all {
		if reason := inv.check(in, ctx); reason != "" {
			out = append(out, Result{ID: inv.id, Reason: reason})
		}
	}
	return out
}

// credentials is INV-01. Credentials never reach the agent. It denies when the
// observed action carries a credential-shaped string. Applies with or without
// declared intent. Declaring "credential" as allowed does not lift it.
func credentials(in input.Input, _ Context) string {
	for _, c := range in.Observed.DataClasses {
		if strings.EqualFold(c, string(dataclass.Credential)) {
			return "credential-shaped value in observed action"
		}
	}
	return ""
}

// egress is INV-02. Egress only to declared destinations. Applies once any
// declared intent is set. Reuses input.DestinationViolation.
func egress(in input.Input, _ Context) string {
	return in.DestinationViolation()
}

// personalDest is INV-03. Personal data never goes to an undeclared
// destination. Applies once any declared intent is set.
func personalDest(in input.Input, _ Context) string {
	if in.Declared.IsZero() || in.DestinationDeclared() {
		return ""
	}
	var personal []string
	for _, c := range in.Observed.DataClasses {
		if dataclass.IsPersonal(c) {
			personal = append(personal, strings.ToLower(c))
		}
	}
	if len(personal) == 0 {
		return ""
	}
	sort.Strings(personal)
	return fmt.Sprintf("personal data (%s) to undeclared destination %q",
		strings.Join(personal, ","), input.Host(in.Observed.Destination))
}

// dataClassSubset is INV-04. Observed data classes must be a subset of the
// declared allowed data classes. Applies once any declared intent is set.
func dataClassSubset(in input.Input, _ Context) string {
	return in.DataClassViolation()
}

// receipts is INV-05. When receipts are required, a missing receipt writer
// fails closed. ReceiptsRequired defaults to false until receipts ship.
func receipts(_ input.Input, ctx Context) string {
	if ctx.ReceiptsRequired && !ctx.ReceiptWriterAvailable {
		return "receipts required but no receipt writer available"
	}
	return ""
}
