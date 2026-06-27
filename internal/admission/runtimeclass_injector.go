package admission

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
	ctrladmission "sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	DefaultRuntimeClassName  = "gvisor"
	DefaultRuntimeLabelKey   = "agentic.clawdlinux.org/runtime-sandbox"
	DefaultRuntimeLabelValue = "gvisor"
)

type RuntimeClassInjectionConfig struct {
	RuntimeClassName string
	LabelKey         string
	LabelValue       string
}

type RuntimeClassInjector struct {
	Config RuntimeClassInjectionConfig
}

func RuntimeClassInjectionConfigFromEnv() RuntimeClassInjectionConfig {
	return RuntimeClassInjectionConfig{
		RuntimeClassName: envOrDefault("RUNTIME_SANDBOX_CLASS", DefaultRuntimeClassName),
		LabelKey:         envOrDefault("RUNTIME_SANDBOX_LABEL_KEY", DefaultRuntimeLabelKey),
		LabelValue:       envOrDefault("RUNTIME_SANDBOX_LABEL_VALUE", DefaultRuntimeLabelValue),
	}
}

func InjectRuntimeClass(pod *corev1.Pod, config RuntimeClassInjectionConfig) bool {
	if pod == nil {
		return false
	}

	runtimeClassName := strings.TrimSpace(config.RuntimeClassName)
	labelKey := strings.TrimSpace(config.LabelKey)
	labelValue := strings.TrimSpace(config.LabelValue)
	if runtimeClassName == "" || labelKey == "" || labelValue == "" {
		return false
	}
	if pod.Labels[labelKey] != labelValue {
		return false
	}
	if pod.Spec.RuntimeClassName != nil && strings.TrimSpace(*pod.Spec.RuntimeClassName) != "" {
		return false
	}

	pod.Spec.RuntimeClassName = &runtimeClassName
	return true
}

func (i *RuntimeClassInjector) Handle(ctx context.Context, req ctrladmission.Request) ctrladmission.Response {
	pod := &corev1.Pod{}
	if err := json.Unmarshal(req.Object.Raw, pod); err != nil {
		return ctrladmission.Errored(http.StatusBadRequest, err)
	}

	mutated := pod.DeepCopy()
	if !InjectRuntimeClass(mutated, i.Config) {
		return ctrladmission.Allowed("runtimeClass injection skipped")
	}

	mutatedRaw, err := json.Marshal(mutated)
	if err != nil {
		return ctrladmission.Errored(http.StatusInternalServerError, err)
	}

	return ctrladmission.PatchResponseFromRaw(req.Object.Raw, mutatedRaw)
}

func envOrDefault(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}
