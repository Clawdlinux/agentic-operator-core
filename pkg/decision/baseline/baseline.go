/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

// Package baseline is a hand-weighted, non-learned scorer. It is the control
// a learned model must beat in offline evaluation. Weights are additive
// micro-units, set by hand, not fitted to any data.
package baseline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"

	"github.com/Clawdlinux/agentic-operator-core/pkg/decision"
)

// ID and Version identify the baseline in scores and records.
const (
	ID      = "clawdlinux-baseline-additive"
	Version = "1"
)

// ThresholdMicro escalates at 0.5.
const ThresholdMicro = 500_000

// Weights are micro-units added per unit of each feature. Risk is the sum,
// clamped to 0..1000000. Features not listed add nothing.
var Weights = map[string]int64{
	"dc_credential":          1_000_000, // INV-01 denies these anyway
	"personal_to_undeclared": 700_000,
	"dc_undeclared_count":    200_000,
	"dc_personal_count":      50_000,
	"decision_automated":     150_000,
	"action_destructive":     300_000,
	"prior_denied":           60_000,
	"conf_absent":            150_000,
	"conf_low":               300_000,
	"conf_mid":               100_000,
	"health_low":             150_000,
	"health_mid":             50_000,
	"declared_present":       -100_000,
	"dest_declared":          -50_000,
}

// Scorer is the baseline. The zero value is ready.
type Scorer struct{}

var _ decision.Scorer = Scorer{}

// SHA256 identifies the weight table. It stands in for an artifact hash.
func SHA256() string {
	names := make([]string, 0, len(Weights))
	for n := range Weights {
		names = append(names, n)
	}
	sort.Strings(names)
	var sb strings.Builder
	sb.WriteString(ID + "/" + Version + "\n")
	for _, n := range names {
		sb.WriteString(n + "=" + strconv.FormatInt(Weights[n], 10) + "\n")
	}
	sum := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(sum[:])
}

// Score adds the weights of the present features.
func (Scorer) Score(_ context.Context, f decision.Features) (decision.Score, error) {
	type contrib struct {
		name string
		v    int64
	}
	var risk int64
	var cs []contrib
	for i, n := range decision.FeatureNames {
		if i >= len(f.Values) {
			break
		}
		c := Weights[n] * f.Values[i]
		risk += c
		if c > 0 {
			cs = append(cs, contrib{n, c})
		}
	}
	sort.Slice(cs, func(i, j int) bool {
		if cs[i].v != cs[j].v {
			return cs[i].v > cs[j].v
		}
		return cs[i].name < cs[j].name
	})
	codes := []string{}
	for i := 0; i < len(cs) && i < 3; i++ {
		codes = append(codes, "feature:"+cs[i].name)
	}
	risk = decision.ClampMicro(risk)
	return decision.Score{
		ModelID:        ID,
		Version:        Version,
		ArtifactSHA256: SHA256(),
		FeatureSpec:    f.SpecVersion,
		FeatureHash:    f.Hash(),
		RiskMicro:      risk,
		OptionMicro:    decision.Options(risk),
		ReasonCodes:    codes,
		ThresholdMicro: ThresholdMicro,
		Calibration:    "uncalibrated: hand-set additive weights, not a probability",
	}, nil
}
