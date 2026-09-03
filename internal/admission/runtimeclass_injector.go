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
	DefaultRuntimeClassName   = "gvisor"
	DefaultRuntimeLabelKey    = "agentic.clawdlinux.org/runtime-sandbox"
	DefaultRuntimeLabelValue  = "gvisor"
	EnforcementModeStrict     = "strict"
	EnforcementModeBestEffort = "best-effort"
)

type RuntimeClassInjectionConfig struct {
	RuntimeClassName string
	LabelKey         string
	LabelValue       string
	EnforcementMode  string
}

type RuntimeClassInjector struct {
	Config  RuntimeClassInjectionConfig
	Checker SandboxReadinessChecker
}

func RuntimeClassInjectionConfigFromEnv() RuntimeClassInjectionConfig {
	return RuntimeClassInjectionConfig{
		RuntimeClassName: envOrDefault("RUNTIME_SANDBOX_CLASS", DefaultRuntimeClassName),
		LabelKey:         envOrDefault("RUNTIME_SANDBOX_LABEL_KEY", DefaultRuntimeLabelKey),
		LabelValue:       envOrDefault("RUNTIME_SANDBOX_LABEL_VALUE", DefaultRuntimeLabelValue),
		EnforcementMode:  enforcementMode(envOrDefault("RUNTIME_SANDBOX_ENFORCEMENT_MODE", EnforcementModeStrict)),
	}
}

func InjectRuntimeClass(pod *corev1.Pod, config RuntimeClassInjectionConfig) bool {
	if !shouldInjectRuntimeClass(pod, config) {
		return false
	}
	runtimeClassName := strings.TrimSpace(config.RuntimeClassName)
	pod.Spec.RuntimeClassName = &runtimeClassName
	return true
}

func shouldInjectRuntimeClass(pod *corev1.Pod, config RuntimeClassInjectionConfig) bool {
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
	return pod.Spec.RuntimeClassName == nil || strings.TrimSpace(*pod.Spec.RuntimeClassName) == ""
}

func (i *RuntimeClassInjector) Handle(ctx context.Context, req ctrladmission.Request) ctrladmission.Response {
	pod := &corev1.Pod{}
	if err := json.Unmarshal(req.Object.Raw, pod); err != nil {
		return ctrladmission.Errored(http.StatusBadRequest, err)
	}

	mutated := pod.DeepCopy()
	if shouldInjectRuntimeClass(pod, i.Config) {
		if i.Checker == nil {
			return i.readinessFailureResponse("sandbox readiness checker is not configured")
		}
		ready, reason, err := i.Checker.IsReady(ctx)
		if err != nil {
			return i.readinessFailureResponse(reason + ": " + err.Error())
		}
		if !ready {
			return i.readinessFailureResponse(reason)
		}
	}
	if !InjectRuntimeClass(mutated, i.Config) {
		return ctrladmission.Allowed("runtimeClass injection skipped")
	}

	mutatedRaw, err := json.Marshal(mutated)
	if err != nil {
		return ctrladmission.Errored(http.StatusInternalServerError, err)
	}

	return ctrladmission.PatchResponseFromRaw(req.Object.Raw, mutatedRaw)
}

func (i *RuntimeClassInjector) readinessFailureResponse(reason string) ctrladmission.Response {
	if enforcementMode(i.Config.EnforcementMode) == EnforcementModeBestEffort {
		return ctrladmission.Allowed("sandbox readiness unverified: " + reason)
	}
	return ctrladmission.Denied("sandbox readiness unverified: " + reason)
}

func enforcementMode(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), EnforcementModeBestEffort) {
		return EnforcementModeBestEffort
	}
	return EnforcementModeStrict
}

func envOrDefault(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}
