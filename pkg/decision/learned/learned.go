/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

// Package learned runs a logistic regression decision model from a JSON
// artifact, in process, with integer-only inference. The same artifact and
// features give the same RiskMicro on every architecture. See
// docs/architecture/decision-model.md.
package learned

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/Clawdlinux/agentic-operator-core/pkg/decision"
)

// SchemaVersion is the artifact schema version this package reads.
const SchemaVersion = 1

// ModelType is the only supported model type.
const ModelType = "logistic_regression"

// MaxArtifactBytes caps the artifact file.
const MaxArtifactBytes = 1 << 20

// Load errors. Each one means "no model": callers fail closed to the
// non-model decision.
var (
	ErrSpecMismatch = errors.New("learned: artifact feature spec does not match this operator")
	ErrTampered     = errors.New("learned: artifact_sha256 does not match the artifact")
	ErrInvalid      = errors.New("learned: invalid artifact")
)

// Bin is one calibration bin from training. Rates are micro-units.
type Bin struct {
	LowerMicro         int64 `json:"lower_micro"`
	UpperMicro         int64 `json:"upper_micro"`
	Count              int64 `json:"count"`
	MeanPredictedMicro int64 `json:"mean_predicted_micro"`
	ObservedMicro      int64 `json:"observed_micro"`
}

// Calibration is the reliability table measured on the held-out split.
type Calibration struct {
	Method   string `json:"method"`
	Split    string `json:"split"`
	Bins     []Bin  `json:"bins"`
	ECEMicro int64  `json:"ece_micro"`
}

// TrainedOn says what the model was fitted to. DataSource is "synthetic",
// "human", or "mixed".
type TrainedOn struct {
	DataSource      string `json:"data_source"`
	DatasetSHA256   string `json:"dataset_sha256"`
	ExamplesTotal   int64  `json:"examples_total"`
	ExamplesTrain   int64  `json:"examples_train"`
	ExamplesHoldout int64  `json:"examples_holdout"`
	Positives       int64  `json:"positives"`
	Negatives       int64  `json:"negatives"`
	SyntheticRows   int64  `json:"synthetic_rows"`
	HumanRows       int64  `json:"human_rows"`
}

// Artifact is the model file. Weights and bias are decimal strings with at
// most 6 fractional digits. No floats anywhere.
type Artifact struct {
	SchemaVersion      int               `json:"schema_version"`
	ModelID            string            `json:"model_id"`
	Version            string            `json:"version"`
	ModelType          string            `json:"model_type"`
	FeatureSpecVersion string            `json:"feature_spec_version"`
	Features           []string          `json:"features"`
	Weights            []string          `json:"weights"`
	Bias               string            `json:"bias"`
	ThresholdMicro     int64             `json:"threshold_micro"`
	Calibration        Calibration       `json:"calibration"`
	TrainedOn          TrainedOn         `json:"trained_on"`
	Training           map[string]string `json:"training"`
	ArtifactSHA256     string            `json:"artifact_sha256"`
}

// Model is a loaded, checked artifact.
type Model struct {
	art     Artifact
	weights []int64
	bias    int64
}

var _ decision.Scorer = (*Model)(nil)

// Load reads and checks an artifact file.
func Load(path string) (*Model, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(io.LimitReader(f, MaxArtifactBytes+1)); err != nil {
		return nil, err
	}
	if buf.Len() > MaxArtifactBytes {
		return nil, fmt.Errorf("%w: larger than %d bytes", ErrInvalid, MaxArtifactBytes)
	}
	return Parse(buf.Bytes())
}

// Parse checks an artifact: printable ASCII only, known schema and model
// type, the exact feature spec and order of this operator, decimal weights,
// a threshold in range, and a matching artifact_sha256.
func Parse(data []byte) (*Model, error) {
	for _, c := range data {
		if (c < 0x20 || c > 0x7e) && c != '\n' && c != '\r' && c != '\t' && c != ' ' {
			return nil, fmt.Errorf("%w: artifact must be printable ASCII", ErrInvalid)
		}
	}
	sum, err := CanonicalSHA256(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	var a Artifact
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&a); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if a.ArtifactSHA256 != sum {
		return nil, ErrTampered
	}
	switch {
	case a.SchemaVersion != SchemaVersion:
		return nil, fmt.Errorf("%w: schema_version %d", ErrInvalid, a.SchemaVersion)
	case a.ModelType != ModelType:
		return nil, fmt.Errorf("%w: model_type %q", ErrInvalid, a.ModelType)
	case a.ModelID == "" || a.Version == "":
		return nil, fmt.Errorf("%w: model_id and version are required", ErrInvalid)
	case a.FeatureSpecVersion != decision.FeatureSpecVersion:
		return nil, fmt.Errorf("%w: artifact %q, operator %q", ErrSpecMismatch, a.FeatureSpecVersion, decision.FeatureSpecVersion)
	case !equal(a.Features, decision.FeatureNames):
		return nil, fmt.Errorf("%w: feature names or order differ", ErrSpecMismatch)
	case len(a.Weights) != len(a.Features):
		return nil, fmt.Errorf("%w: %d weights for %d features", ErrInvalid, len(a.Weights), len(a.Features))
	case a.ThresholdMicro < 1 || a.ThresholdMicro > decision.MicroScale:
		return nil, fmt.Errorf("%w: threshold_micro %d", ErrInvalid, a.ThresholdMicro)
	}
	m := &Model{art: a, weights: make([]int64, len(a.Weights))}
	for i, w := range a.Weights {
		if m.weights[i], err = ParseMicro(w); err != nil {
			return nil, fmt.Errorf("%w: weight %s: %v", ErrInvalid, a.Features[i], err)
		}
	}
	if m.bias, err = ParseMicro(a.Bias); err != nil {
		return nil, fmt.Errorf("%w: bias: %v", ErrInvalid, err)
	}
	return m, nil
}

// CanonicalSHA256 hashes the artifact with artifact_sha256 removed, as
// compact JSON with sorted keys and numbers kept as written. Python
// json.dumps(sort_keys=True, separators=(",", ":")) gives the same bytes for
// a printable ASCII artifact with integer numbers.
func CanonicalSHA256(data []byte) (string, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var obj map[string]any
	if err := dec.Decode(&obj); err != nil {
		return "", err
	}
	if dec.More() {
		return "", errors.New("trailing data")
	}
	delete(obj, "artifact_sha256")
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(obj); err != nil {
		return "", err
	}
	sum := sha256.Sum256(bytes.TrimSuffix(buf.Bytes(), []byte("\n")))
	return hex.EncodeToString(sum[:]), nil
}

// Artifact returns the loaded artifact.
func (m *Model) Artifact() Artifact { return m.art }

// ThresholdMicro is the artifact escalation threshold.
func (m *Model) ThresholdMicro() int64 { return m.art.ThresholdMicro }

// Logit returns bias + sum(weight * value) in micro-units. Integer only.
func (m *Model) Logit(f decision.Features) (int64, error) {
	if f.SpecVersion != m.art.FeatureSpecVersion || len(f.Values) != len(m.weights) {
		return 0, fmt.Errorf("%w: features %s/%d, model %s/%d", ErrSpecMismatch, f.SpecVersion, len(f.Values), m.art.FeatureSpecVersion, len(m.weights))
	}
	z := m.bias
	for i, w := range m.weights {
		z += w * f.Values[i]
	}
	return z, nil
}

// Score returns the risk that a human would reject or edit the action.
func (m *Model) Score(_ context.Context, f decision.Features) (decision.Score, error) {
	z, err := m.Logit(f)
	if err != nil {
		return decision.Score{}, err
	}
	risk := SigmoidMicro(z)
	type contrib struct {
		name string
		v    int64
	}
	var cs []contrib
	for i, w := range m.weights {
		if c := w * f.Values[i]; c > 0 {
			cs = append(cs, contrib{m.art.Features[i], c})
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
	cal := m.art.Calibration
	return decision.Score{
		ModelID:        m.art.ModelID,
		Version:        m.art.Version,
		ArtifactSHA256: m.art.ArtifactSHA256,
		FeatureSpec:    f.SpecVersion,
		FeatureHash:    f.Hash(),
		RiskMicro:      risk,
		OptionMicro:    decision.Options(risk),
		ReasonCodes:    codes,
		ThresholdMicro: m.art.ThresholdMicro,
		Calibration: fmt.Sprintf("%s; %s ECE %d micro over %d bins; trained on %s data",
			nonEmpty(cal.Method, "none"), nonEmpty(cal.Split, "holdout"), cal.ECEMicro, len(cal.Bins), nonEmpty(m.art.TrainedOn.DataSource, "unknown")),
	}, nil
}

// ParseMicro parses a decimal string with at most 6 fractional digits into
// micro-units. Magnitude is capped at 1000 to keep sums far from overflow.
func ParseMicro(s string) (int64, error) {
	neg := strings.HasPrefix(s, "-")
	body := strings.TrimPrefix(s, "-")
	ip, fr, hasDot := strings.Cut(body, ".")
	if ip == "" || (hasDot && fr == "") || len(fr) > 6 || len(ip) > 4 {
		return 0, fmt.Errorf("bad decimal %q", s)
	}
	for _, c := range ip + fr {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("bad decimal %q", s)
		}
	}
	whole, err := strconv.ParseInt(ip, 10, 64)
	if err != nil {
		return 0, err
	}
	fr += strings.Repeat("0", 6-len(fr))
	frac, err := strconv.ParseInt(fr, 10, 64)
	if err != nil {
		return 0, err
	}
	v := whole*decision.MicroScale + frac
	if v > 1000*decision.MicroScale {
		return 0, fmt.Errorf("decimal %q out of range", s)
	}
	if neg {
		v = -v
	}
	return v, nil
}

// Fixed-point constants for SigmoidMicro. expScale is 1.0.
const (
	expScale = 1_000_000_000
	// eInv is e^-1 at expScale, rounded down.
	eInv = 367_879_441
	// maxLogitMicro clamps the logit. e^-30 is below one expScale unit.
	maxLogitMicro = 30 * decision.MicroScale
)

// expNeg returns e^(-x) at expScale for x >= 0 in micro-units, with integer
// arithmetic only: e^(-n) by repeated multiplication with eInv, e^(-f) for
// the fraction by a 25-term Taylor series. Every intermediate product stays
// below 1e18, inside int64.
func expNeg(xMicro int64) int64 {
	n := xMicro / decision.MicroScale
	f := (xMicro % decision.MicroScale) * 1000 // fraction at expScale
	sum, term := int64(expScale), int64(expScale)
	for k := int64(1); k <= 25; k++ {
		term = term * f / (expScale * k)
		if term == 0 {
			break
		}
		if k%2 == 1 {
			sum -= term
		} else {
			sum += term
		}
	}
	for i := int64(0); i < n; i++ {
		sum = sum * eInv / expScale
	}
	return sum
}

// SigmoidMicro returns round(1e6 / (1 + e^-z)) for a logit z in micro-units.
// Integer arithmetic only, so the result does not depend on the platform
// floating point unit or math library.
func SigmoidMicro(zMicro int64) int64 {
	if zMicro > maxLogitMicro {
		zMicro = maxLogitMicro
	}
	if zMicro < -maxLogitMicro {
		zMicro = -maxLogitMicro
	}
	neg := zMicro < 0
	if neg {
		zMicro = -zMicro
	}
	e := expNeg(zMicro)
	den := int64(expScale) + e
	if neg {
		return (e*decision.MicroScale + den/2) / den
	}
	return (int64(expScale)*decision.MicroScale + den/2) / den
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func nonEmpty(s, d string) string {
	if s == "" {
		return d
	}
	return s
}
