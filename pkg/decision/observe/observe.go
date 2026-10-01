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

// Package observe builds observed decision inputs from facts the controller
// already holds. It is pure: no cluster, no network.
package observe

import (
	agenticv1alpha1 "github.com/Clawdlinux/agentic-operator-core/api/v1alpha1"
	"github.com/Clawdlinux/agentic-operator-core/pkg/dataclass"
	"github.com/Clawdlinux/agentic-operator-core/pkg/decision/input"
)

// Facts are plain values the controller passes in.
type Facts struct {
	// Endpoint is the URL the action will be sent to. Only the host is kept.
	Endpoint string
	// Tool is the action or tool name being called.
	Tool string
	// CallerIdentity is the identity that triggered the action, if known.
	CallerIdentity string
	// Payload is scanned for data classes. Matched content is never kept.
	Payload any
	// PriorActionCount and PriorDeniedCount come from workload status.
	PriorActionCount int
	PriorDeniedCount int
}

// Observe converts facts into an input.Observed.
func Observe(f Facts) input.Observed {
	return input.Observed{
		Destination:      input.Host(f.Endpoint),
		DataClasses:      dataclass.Strings(dataclass.DetectValue(f.Payload)),
		Tool:             f.Tool,
		CallerIdentity:   f.CallerIdentity,
		PriorActionCount: f.PriorActionCount,
		PriorDeniedCount: f.PriorDeniedCount,
	}
}

// History counts prior actions recorded in workload status. total is executed
// plus proposed. denied is proposed actions marked not approved.
func History(executed, proposed []agenticv1alpha1.Action) (total, denied int) {
	total = len(executed) + len(proposed)
	for _, a := range proposed {
		if a.Approved != nil && !*a.Approved {
			denied++
		}
	}
	return total, denied
}

// Declared converts the CRD declared intent into an input.Declared. A nil
// intent yields the zero value, which means undeclared.
func Declared(d *agenticv1alpha1.DeclaredIntent) input.Declared {
	if d == nil {
		return input.Declared{}
	}
	return input.Declared{
		Purpose:             d.Purpose,
		DecisionType:        d.DecisionType,
		AllowedDataClasses:  append([]string(nil), d.AllowedDataClasses...),
		AllowedDestinations: append([]string(nil), d.AllowedDestinations...),
	}
}
