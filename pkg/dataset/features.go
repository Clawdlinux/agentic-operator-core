/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package dataset

import (
	"strconv"

	"github.com/Clawdlinux/agentic-operator-core/pkg/decision"
	"github.com/Clawdlinux/agentic-operator-core/pkg/decision/input"
)

// FeatureInput maps an example to the decision model feature input, so
// training and offline evaluation use the same features as live scoring.
// The action name comes from Content.Name, which capture mode none omits:
// such examples always get action_destructive 0.
func (ex Example) FeatureInput() decision.FeatureInput {
	f := ex.Features
	return decision.FeatureInput{
		Input: input.Input{
			Declared: input.Declared{
				DecisionType:        f.DecisionType,
				AllowedDataClasses:  f.AllowedDataClasses,
				AllowedDestinations: f.AllowedDestinations,
			},
			Observed: input.Observed{
				Destination:      f.ObservedDestination,
				DataClasses:      f.ObservedDataClasses,
				Tool:             f.Tool,
				PriorActionCount: f.PriorActionCount,
				PriorDeniedCount: f.PriorDeniedCount,
			},
			Claimed: input.AgentClaimed{
				Confidence:    parseDecimal(f.ClaimedConfidence),
				ClusterHealth: parseDecimal(f.ClaimedClusterHealth),
			},
		},
		Action:      ex.Content.Name,
		PolicyPacks: f.PolicyPacks,
		EscalatedBy: f.EscalatedBy,
	}
}

// Rejected reports whether the human label counts as a positive for the
// escalate model: reject or edit.
func (ex Example) Rejected() bool {
	return ex.Label == "reject" || ex.Label == "edit"
}

func parseDecimal(s string) *float64 {
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &v
}
