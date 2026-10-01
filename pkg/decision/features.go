/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package decision

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/Clawdlinux/agentic-operator-core/pkg/dataclass"
	"github.com/Clawdlinux/agentic-operator-core/pkg/decision/input"
)

// FeatureSpecVersion names the feature list below. Any change to a name, an
// order, a bucket edge, or a keyword list needs a new version. A model artifact
// trained on another version is refused. tools/decision-train/features.py
// mirrors this file and must change with it.
const FeatureSpecVersion = "v1"

// FeatureNames is the v1 feature order. Every value is a small integer.
var FeatureNames = []string{
	"dc_personal_count",      // distinct personal classes observed, capped at 6
	"dc_credential",          // credential class observed
	"dc_undeclared_count",    // observed classes outside the declared list, capped at 6
	"personal_to_undeclared", // personal data and a destination outside the declared list
	"dest_present",           // a destination host was observed
	"dest_declared",          // the destination host is in the declared list
	"declared_present",       // decision type or an allow list is declared
	"decision_automated",     // declared decision type is "automated"
	"action_destructive",     // action name has a destructive keyword token
	"prior_actions_bucket",   // 0, 1-9, 10-99, 100+ as 0..3
	"prior_denied",           // prior denied count, capped at 10
	"conf_absent",            // no claimed confidence
	"conf_low",               // claimed confidence below 0.5
	"conf_mid",               // claimed confidence 0.5 to below 0.8
	"health_absent",          // no claimed cluster health
	"health_low",             // claimed cluster health below 50
	"health_mid",             // claimed cluster health 50 to below 80
	"pack_any",               // any policy pack set
	"pack_dpdp_in",           // dpdp-in pack set
	"pack_gdpr_eu",           // gdpr-eu pack set
	"esc_invariant",          // a non-allow outcome came from invariants
	"esc_rules",              // a non-allow outcome came from policy packs
	"esc_threshold",          // a non-allow outcome came from the threshold
	"esc_model",              // a non-allow outcome came from the model
}

// DestructiveKeywords marks an action name as destructive when any token of
// the name, split on _ - . / and space, equals one of them.
var DestructiveKeywords = []string{"cleanup", "clear", "delete", "destroy", "drop", "purge", "remove", "reset", "truncate", "wipe"}

// FeatureInput is what features are derived from. Live decisions and dataset
// examples both map to it, so training and serving see the same values.
type FeatureInput struct {
	Input       input.Input
	Action      string
	PolicyPacks []string
	// EscalatedBy is the record layer (invariant, rules, threshold, model)
	// of a non-allow outcome. Empty when the outcome was allow.
	EscalatedBy string
}

// Features is one feature vector at FeatureSpecVersion. Values follows
// FeatureNames.
type Features struct {
	SpecVersion string
	Values      []int64
}

// ExtractFeatures derives the feature vector. Same input, same vector.
// Declared purpose is not used: dataset examples do not carry it.
func ExtractFeatures(fi FeatureInput) Features {
	d, o, c := fi.Input.Declared, fi.Input.Observed, fi.Input.Claimed
	declared := d.DecisionType != "" || len(d.AllowedDataClasses) > 0 || len(d.AllowedDestinations) > 0

	personal := map[string]bool{}
	credential := false
	for _, cl := range o.DataClasses {
		l := strings.ToLower(strings.TrimSpace(cl))
		if dataclass.IsPersonal(l) {
			personal[l] = true
		}
		if l == string(dataclass.Credential) {
			credential = true
		}
	}
	undeclared := 0
	if declared {
		allowed := map[string]bool{}
		for _, a := range d.AllowedDataClasses {
			allowed[strings.ToLower(strings.TrimSpace(a))] = true
		}
		seen := map[string]bool{}
		for _, cl := range o.DataClasses {
			l := strings.ToLower(strings.TrimSpace(cl))
			if !allowed[l] && !seen[l] {
				seen[l] = true
				undeclared++
			}
		}
	}
	host := input.Host(o.Destination)
	destDeclared := host != "" && input.Input{
		Declared: input.Declared{AllowedDestinations: d.AllowedDestinations},
		Observed: input.Observed{Destination: host},
	}.DestinationDeclared()

	v := map[string]int64{
		"dc_personal_count":      capInt(len(personal), 6),
		"dc_credential":          b(credential),
		"dc_undeclared_count":    capInt(undeclared, 6),
		"personal_to_undeclared": b(len(personal) > 0 && host != "" && !destDeclared),
		"dest_present":           b(host != ""),
		"dest_declared":          b(destDeclared),
		"declared_present":       b(declared),
		"decision_automated":     b(strings.EqualFold(strings.TrimSpace(d.DecisionType), "automated")),
		"action_destructive":     b(IsDestructive(fi.Action)),
		"prior_actions_bucket":   bucketCount(o.PriorActionCount),
		"prior_denied":           capInt(o.PriorDeniedCount, 10),
	}
	switch {
	case c.Confidence == nil:
		v["conf_absent"] = 1
	case *c.Confidence < 0.5:
		v["conf_low"] = 1
	case *c.Confidence < 0.8:
		v["conf_mid"] = 1
	}
	switch {
	case c.ClusterHealth == nil:
		v["health_absent"] = 1
	case *c.ClusterHealth < 50:
		v["health_low"] = 1
	case *c.ClusterHealth < 80:
		v["health_mid"] = 1
	}
	for _, p := range fi.PolicyPacks {
		v["pack_any"] = 1
		switch name, _, _ := strings.Cut(strings.ToLower(p), "@"); name {
		case "dpdp-in":
			v["pack_dpdp_in"] = 1
		case "gdpr-eu":
			v["pack_gdpr_eu"] = 1
		}
	}
	switch fi.EscalatedBy {
	case "invariant":
		v["esc_invariant"] = 1
	case "rules", LayerPack:
		v["esc_rules"] = 1
	case LayerThreshold:
		v["esc_threshold"] = 1
	case LayerModel:
		v["esc_model"] = 1
	}
	out := Features{SpecVersion: FeatureSpecVersion, Values: make([]int64, len(FeatureNames))}
	for i, n := range FeatureNames {
		out.Values[i] = v[n]
	}
	return out
}

// Hash is the SHA-256 hex of "clawdlinux-features/<version>\n" followed by
// one "name=value\n" line per feature in order.
func (f Features) Hash() string {
	var sb strings.Builder
	sb.WriteString("clawdlinux-features/" + f.SpecVersion + "\n")
	for i, n := range FeatureNames {
		if i < len(f.Values) {
			sb.WriteString(n + "=" + strconv.FormatInt(f.Values[i], 10) + "\n")
		}
	}
	sum := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(sum[:])
}

// Value returns the named feature, or 0 when unknown.
func (f Features) Value(name string) int64 {
	for i, n := range FeatureNames {
		if n == name && i < len(f.Values) {
			return f.Values[i]
		}
	}
	return 0
}

// IsDestructive reports whether a token of name is a destructive keyword.
func IsDestructive(name string) bool {
	for _, tok := range strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
		return r == '_' || r == '-' || r == '.' || r == '/' || r == ' '
	}) {
		for _, k := range DestructiveKeywords {
			if tok == k {
				return true
			}
		}
	}
	return false
}

func bucketCount(n int) int64 {
	switch {
	case n <= 0:
		return 0
	case n < 10:
		return 1
	case n < 100:
		return 2
	}
	return 3
}

func capInt(n, max int) int64 {
	if n < 0 {
		return 0
	}
	if n > max {
		return int64(max)
	}
	return int64(n)
}

func b(x bool) int64 {
	if x {
		return 1
	}
	return 0
}
