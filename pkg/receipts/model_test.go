/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package receipts

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Clawdlinux/agentgate/pkg/receiptspec"

	"github.com/Clawdlinux/agentic-operator-core/pkg/decision"
)

func TestModelBlockRecord(t *testing.T) {
	s := decision.Score{
		ModelID: "m", Version: "1", ArtifactSHA256: strings.Repeat("a", 64), FeatureSpec: "v1", FeatureHash: strings.Repeat("b", 64),
		RiskMicro: 700000, OptionMicro: decision.Options(700000), ReasonCodes: []string{"feature:conf_low"},
		ThresholdMicro: 500000, Calibration: "none",
	}
	base := decision.Result{Outcome: decision.Allow, Layer: decision.LayerThreshold}
	final := decision.ApplyModel(base, s, nil, decision.ModelEscalate)
	block := NewModelBlock(decision.ModelEscalate, s, nil, base, final)
	if !reflect.DeepEqual(block.OptionSet, []string{"allow", "require_approval"}) || !reflect.DeepEqual(block.OptionMicro, []int64{300000, 700000}) {
		t.Fatalf("options %v %v", block.OptionSet, block.OptionMicro)
	}
	if block.BaseOutcome != "allow" || block.Outcome != "require_approval" || block.Error != "" || block.SchemaVersion != ModelBlockVersion {
		t.Fatalf("block %+v", block)
	}
	rec, err := NewDecisionRecord(Params{Action: "a", Result: final, Model: &block})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Layer != LayerModel || rec.Outcome != OutcomeRequireApproval {
		t.Fatalf("layer %s outcome %s", rec.Layer, rec.Outcome)
	}
	h1, err := receiptspec.HashRecord(rec)
	if err != nil {
		t.Fatalf("model block must be a valid record: %v", err)
	}
	rec2, _ := NewDecisionRecord(Params{Action: "a", Result: final, Model: &block})
	if h2, _ := receiptspec.HashRecord(rec2); h1 != h2 {
		t.Fatal("record not deterministic")
	}
	raw, _ := json.Marshal(rec)
	if strings.Contains(string(raw), "e+") || strings.Contains(string(raw), ".5") {
		t.Fatalf("float-like number in record: %s", raw)
	}
}

func TestModelBlockError(t *testing.T) {
	base := decision.Result{Outcome: decision.Allow, Layer: decision.LayerThreshold}
	b := NewModelBlock(decision.ModelShadow, decision.Score{}, errors.New("scorer down"), base, base)
	if b.Error != "scorer down" || !reflect.DeepEqual(b.OptionMicro, []int64{0, 0}) || b.Outcome != "allow" {
		t.Fatalf("block %+v", b)
	}
	rec, err := NewDecisionRecord(Params{Action: "a", Result: base, Model: &b})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := receiptspec.HashRecord(rec); err != nil {
		t.Fatal(err)
	}
}

func TestNoModelBlockByDefault(t *testing.T) {
	rec, err := NewDecisionRecord(Params{Action: "a", Result: decision.Result{Outcome: decision.Allow, Layer: decision.LayerThreshold}})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(rec)
	if rec.Model != nil || strings.Contains(string(raw), `"model"`) {
		t.Fatalf("model block present without a model: %s", raw)
	}
}
