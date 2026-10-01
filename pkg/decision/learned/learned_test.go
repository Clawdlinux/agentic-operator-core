/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package learned_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/Clawdlinux/agentic-operator-core/pkg/dataset"
	"github.com/Clawdlinux/agentic-operator-core/pkg/decision"
	"github.com/Clawdlinux/agentic-operator-core/pkg/decision/learned"
)

var outDir = filepath.Join("..", "..", "..", "tools", "decision-train", "out")

func readFile(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(outDir, name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func examples(t *testing.T) []dataset.Example {
	t.Helper()
	var out []dataset.Example
	sc := bufio.NewScanner(bytes.NewReader(readFile(t, "approvals.jsonl")))
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		var ex dataset.Example
		dec := json.NewDecoder(bytes.NewReader(sc.Bytes()))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&ex); err != nil {
			t.Fatal(err)
		}
		out = append(out, ex)
	}
	return out
}

func shipped(t *testing.T) *learned.Model {
	t.Helper()
	m, err := learned.Load(filepath.Join(outDir, "model.json"))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestShippedArtifact(t *testing.T) {
	m := shipped(t)
	a := m.Artifact()
	if a.TrainedOn.DataSource != "synthetic" || a.TrainedOn.HumanRows != 0 {
		t.Fatalf("shipped artifact must say it is synthetic: %+v", a.TrainedOn)
	}
	if !strings.Contains(a.Version, "synthetic") {
		t.Fatalf("version %q must name synthetic data", a.Version)
	}
}

// TestGoldenCrossLanguage checks Go against outputs the Python trainer
// computed with the same integer inference.
func TestGoldenCrossLanguage(t *testing.T) {
	m := shipped(t)
	byID := map[string]dataset.Example{}
	for _, ex := range examples(t) {
		byID[ex.ExampleID] = ex
	}
	var golden []struct {
		ExampleID  string  `json:"example_id"`
		Features   []int64 `json:"features"`
		LogitMicro int64   `json:"logit_micro"`
		RiskMicro  int64   `json:"risk_micro"`
	}
	if err := json.Unmarshal(readFile(t, "golden.json"), &golden); err != nil {
		t.Fatal(err)
	}
	if len(golden) < 20 {
		t.Fatalf("only %d golden cases", len(golden))
	}
	for _, g := range golden {
		if g.ExampleID == "sigmoid" {
			if got := learned.SigmoidMicro(g.LogitMicro); got != g.RiskMicro {
				t.Errorf("SigmoidMicro(%d) = %d, python %d", g.LogitMicro, got, g.RiskMicro)
			}
			continue
		}
		ex, ok := byID[g.ExampleID]
		if !ok {
			t.Fatalf("golden id %s not in dataset", g.ExampleID)
		}
		f := decision.ExtractFeatures(ex.FeatureInput())
		if !reflect.DeepEqual(f.Values, g.Features) {
			t.Fatalf("%s features %v, python %v", g.ExampleID, f.Values, g.Features)
		}
		z, err := m.Logit(f)
		if err != nil || z != g.LogitMicro {
			t.Fatalf("%s logit %d (%v), python %d", g.ExampleID, z, err, g.LogitMicro)
		}
		s, err := m.Score(context.Background(), f)
		if err != nil || s.RiskMicro != g.RiskMicro {
			t.Fatalf("%s risk %d (%v), python %d", g.ExampleID, s.RiskMicro, err, g.RiskMicro)
		}
	}
}

// TestFeatureMatrixParity proves Go and tools/decision-train/features.py
// derive the same features from every shipped example.
func TestFeatureMatrixParity(t *testing.T) {
	var metrics struct {
		FeatureMatrixSHA256 string `json:"feature_matrix_sha256"`
	}
	if err := json.Unmarshal(readFile(t, "metrics.json"), &metrics); err != nil {
		t.Fatal(err)
	}
	h := sha256.New()
	for _, ex := range examples(t) {
		f := decision.ExtractFeatures(ex.FeatureInput())
		parts := make([]string, len(f.Values))
		for i, v := range f.Values {
			parts[i] = strconv.FormatInt(v, 10)
		}
		h.Write([]byte(strings.Join(parts, ",") + "\n"))
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != metrics.FeatureMatrixSHA256 {
		t.Fatalf("feature matrix %s, python %s: features.go and features.py drifted", got, metrics.FeatureMatrixSHA256)
	}
}

func TestReplay(t *testing.T) {
	m := shipped(t)
	exs := examples(t)
	for _, ex := range exs[:40] {
		f := decision.ExtractFeatures(ex.FeatureInput())
		a, err := m.Score(context.Background(), f)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := shipped(t).Score(context.Background(), decision.ExtractFeatures(ex.FeatureInput()))
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("replay differs for %s", ex.ExampleID)
		}
		if a.OptionMicro[decision.OptionAllow]+a.OptionMicro[decision.OptionRequireApproval] != decision.MicroScale {
			t.Fatal("options do not sum")
		}
		if a.ArtifactSHA256 != m.Artifact().ArtifactSHA256 || a.FeatureHash != f.Hash() {
			t.Fatal("score metadata missing")
		}
	}
}

// rewrite edits the shipped artifact. When reseal is set it recomputes
// artifact_sha256, like an honest retrain would.
func rewrite(t *testing.T, edit func(map[string]any), reseal bool) []byte {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(readFile(t, "model.json")))
	dec.UseNumber()
	var obj map[string]any
	if err := dec.Decode(&obj); err != nil {
		t.Fatal(err)
	}
	edit(obj)
	raw, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	if reseal {
		sum, err := learned.CanonicalSHA256(raw)
		if err != nil {
			t.Fatal(err)
		}
		obj["artifact_sha256"] = sum
		if raw, err = json.Marshal(obj); err != nil {
			t.Fatal(err)
		}
	}
	return raw
}

func TestParseRejects(t *testing.T) {
	tests := []struct {
		name   string
		edit   func(map[string]any)
		reseal bool
		want   error
	}{
		{"weight tampered", func(o map[string]any) { o["weights"].([]any)[0] = "9.000000" }, false, learned.ErrTampered},
		{"threshold tampered", func(o map[string]any) { o["threshold_micro"] = json.Number("900000") }, false, learned.ErrTampered},
		{"hash replaced", func(o map[string]any) { o["artifact_sha256"] = strings.Repeat("0", 64) }, false, learned.ErrTampered},
		{"spec version mismatch", func(o map[string]any) { o["feature_spec_version"] = "v0" }, true, learned.ErrSpecMismatch},
		{"feature order mismatch", func(o map[string]any) {
			f := o["features"].([]any)
			f[0], f[1] = f[1], f[0]
		}, true, learned.ErrSpecMismatch},
		{"weights count", func(o map[string]any) { o["weights"] = o["weights"].([]any)[1:] }, true, learned.ErrInvalid},
		{"float weight", func(o map[string]any) { o["weights"].([]any)[0] = "0.1234567" }, true, learned.ErrInvalid},
		{"threshold zero", func(o map[string]any) { o["threshold_micro"] = json.Number("0") }, true, learned.ErrInvalid},
		{"schema version", func(o map[string]any) { o["schema_version"] = json.Number("2") }, true, learned.ErrInvalid},
		{"model type", func(o map[string]any) { o["model_type"] = "mlp" }, true, learned.ErrInvalid},
		{"unknown field", func(o map[string]any) { o["extra"] = "x" }, true, learned.ErrInvalid},
		{"non ascii", func(o map[string]any) { o["model_id"] = "caf\u00e9" }, true, learned.ErrInvalid},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := learned.Parse(rewrite(t, tc.edit, tc.reseal))
			if !errors.Is(err, tc.want) {
				t.Fatalf("Parse err = %v, want %v", err, tc.want)
			}
		})
	}
	if _, err := learned.Parse(rewrite(t, func(map[string]any) {}, true)); err != nil {
		t.Fatalf("resealed unchanged artifact rejected: %v", err)
	}
}

func TestScoreRejectsWrongFeatures(t *testing.T) {
	m := shipped(t)
	for _, f := range []decision.Features{
		{SpecVersion: "v0", Values: make([]int64, len(decision.FeatureNames))},
		{SpecVersion: decision.FeatureSpecVersion, Values: []int64{1}},
	} {
		if _, err := m.Score(context.Background(), f); !errors.Is(err, learned.ErrSpecMismatch) {
			t.Fatalf("Score err = %v", err)
		}
	}
}

func TestSigmoidMicro(t *testing.T) {
	prev := int64(-1)
	for z := int64(-31_000_000); z <= 31_000_000; z += 7_919 {
		r := learned.SigmoidMicro(z)
		if r < 0 || r > decision.MicroScale || r < prev {
			t.Fatalf("SigmoidMicro(%d) = %d not monotonic in range (prev %d)", z, r, prev)
		}
		prev = r
		want := 1e6 / (1 + math.Exp(-float64(z)/1e6))
		if math.Abs(float64(r)-want) > 2 {
			t.Fatalf("SigmoidMicro(%d) = %d, float %f", z, r, want)
		}
		if s := r + learned.SigmoidMicro(-z); s < decision.MicroScale-1 || s > decision.MicroScale+1 {
			t.Fatalf("not symmetric at %d: %d", z, s)
		}
	}
	if learned.SigmoidMicro(0) != 500_000 {
		t.Fatal("sigmoid(0) != 0.5")
	}
}

func TestParseMicro(t *testing.T) {
	tests := []struct {
		in   string
		want int64
		ok   bool
	}{
		{"0", 0, true}, {"1.5", 1_500_000, true}, {"-0.000001", -1, true}, {"-12.345678", -12_345_678, true},
		{"1000", 1_000_000_000, true}, {"1000.000001", 0, false}, {"1e3", 0, false}, {"", 0, false},
		{".5", 0, false}, {"5.", 0, false}, {"0.1234567", 0, false}, {"+1", 0, false}, {"--1", 0, false},
	}
	for _, tc := range tests {
		got, err := learned.ParseMicro(tc.in)
		if (err == nil) != tc.ok || (tc.ok && got != tc.want) {
			t.Errorf("ParseMicro(%q) = %d, %v", tc.in, got, err)
		}
	}
}
