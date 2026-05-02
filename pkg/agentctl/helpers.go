package agentctl

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	sigyaml "sigs.k8s.io/yaml"
)

// NestedString extracts a string from an unstructured map.
func NestedString(obj map[string]interface{}, fields ...string) string {
	value, found, _ := unstructured.NestedString(obj, fields...)
	if !found {
		return ""
	}
	return value
}

// ExtractModel returns the best available model identifier from a workload.
func ExtractModel(obj map[string]interface{}) string {
	if model := NestedString(obj, "spec", "model"); model != "" {
		return model
	}
	if value := NestedString(obj, "status", "model"); value != "" {
		return value
	}
	if providers, found, _ := unstructured.NestedSlice(obj, "spec", "providers"); found && len(providers) > 0 {
		if first, ok := providers[0].(map[string]interface{}); ok {
			name, _ := first["name"].(string)
			typ, _ := first["type"].(string)
			if name != "" && typ != "" {
				return name + "/" + typ
			}
			if name != "" {
				return name
			}
		}
	}
	if mapping, found, _ := unstructured.NestedStringMap(obj, "spec", "modelMapping"); found && len(mapping) > 0 {
		keys := make([]string, 0, len(mapping))
		for k := range mapping {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return mapping[keys[0]]
	}
	return ""
}

// AgeString returns a human-readable age string.
func AgeString(ts metav1.Time) string {
	if ts.IsZero() {
		return "-"
	}
	d := time.Since(ts.Time)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// SafeText returns the value or a fallback if blank.
func SafeText(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

// PrintStructured writes JSON or YAML to a writer.
func PrintStructured(w io.Writer, data interface{}, format string) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	case "yaml":
		b, err := sigyaml.Marshal(data)
		if err != nil {
			return err
		}
		_, err = w.Write(b)
		return err
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

// ValueString extracts a string from a map.
func ValueString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// StringFromMap extracts a string value from a map.
func StringFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// NestedMapString extracts a string from a nested map.
func NestedMapString(m map[string]interface{}, parent, key string) string {
	child, ok := m[parent].(map[string]interface{})
	if !ok {
		return ""
	}
	if v, ok := child[key].(string); ok {
		return v
	}
	return ""
}

// FirstNonEmpty returns the first non-blank string.
func FirstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// Int64FromAny converts the first numeric-like value to int64.
func Int64FromAny(values ...interface{}) int64 {
	for _, val := range values {
		switch t := val.(type) {
		case int:
			return int64(t)
		case int32:
			return int64(t)
		case int64:
			return t
		case float64:
			return int64(t)
		case json.Number:
			if x, err := t.Int64(); err == nil {
				return x
			}
		case string:
			if x, err := strconv.ParseInt(t, 10, 64); err == nil {
				return x
			}
		}
	}
	return 0
}

// Float64FromAny converts the first numeric-like value to float64.
func Float64FromAny(values ...interface{}) float64 {
	for _, val := range values {
		switch t := val.(type) {
		case float64:
			return t
		case float32:
			return float64(t)
		case int:
			return float64(t)
		case int64:
			return float64(t)
		case json.Number:
			if x, err := t.Float64(); err == nil {
				return x
			}
		case string:
			if x, err := strconv.ParseFloat(t, 64); err == nil {
				return x
			}
		}
	}
	return 0
}
