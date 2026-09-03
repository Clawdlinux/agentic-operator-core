package netpolicy

import (
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes/typed/apps/v1"
)

const kubeSystemNamespace = "kube-system"

type Enforcement string

const (
	EnforcementEnforcing         Enforcement = "Enforcing"
	EnforcementKnownNonEnforcing Enforcement = "KnownNonEnforcing"
	EnforcementUnknown           Enforcement = "Unknown"
)

type Detection struct {
	Enforcement Enforcement
	Reason      string
}

type DaemonSetClient interface {
	List(ctx context.Context, opts metav1.ListOptions) (*appsv1.DaemonSetList, error)
}

type DiscoveryClient interface {
	ServerGroups() (*metav1.APIGroupList, error)
}

// DetectCNIEnforcement classifies NetworkPolicy enforcement from known CNI API
// groups and kube-system DaemonSet names. It does not actively test packet flow.
func DetectCNIEnforcement(ctx context.Context, discoveryClient DiscoveryClient, daemonSets DaemonSetClient) (Detection, error) {
	if discoveryClient == nil || daemonSets == nil {
		return Detection{}, fmt.Errorf("network policy detection requires discovery and DaemonSet clients")
	}

	groups, err := discoveryClient.ServerGroups()
	if err != nil {
		return Detection{}, fmt.Errorf("detect NetworkPolicy CNI: list API groups: %w", err)
	}
	if groups == nil {
		return Detection{}, fmt.Errorf("detect NetworkPolicy CNI: list API groups returned no result")
	}
	if groupPresent(groups.Groups, "cilium.io") {
		return Detection{Enforcement: EnforcementEnforcing, Reason: "Cilium API group detected"}, nil
	}
	if groupPresent(groups.Groups, "crd.projectcalico.org") {
		return Detection{Enforcement: EnforcementEnforcing, Reason: "Calico API group detected"}, nil
	}

	daemonSetList, err := daemonSets.List(ctx, metav1.ListOptions{})
	if err != nil {
		return Detection{}, fmt.Errorf("detect NetworkPolicy CNI: list kube-system DaemonSets: %w", err)
	}
	if daemonSetList == nil {
		return Detection{}, fmt.Errorf("detect NetworkPolicy CNI: list kube-system DaemonSets returned no result")
	}
	if daemonSetNamed(daemonSetList.Items, "cilium") || daemonSetNamed(daemonSetList.Items, "calico-node") || daemonSetNamed(daemonSetList.Items, "antrea-agent") || daemonSetNamed(daemonSetList.Items, "weave-net") {
		return Detection{Enforcement: EnforcementEnforcing, Reason: "NetworkPolicy-enforcing CNI DaemonSet detected"}, nil
	}
	if daemonSetNamed(daemonSetList.Items, "kindnet") || daemonSetNamed(daemonSetList.Items, "kube-flannel-ds") || daemonSetNamed(daemonSetList.Items, "flannel") {
		return Detection{Enforcement: EnforcementKnownNonEnforcing, Reason: "CNI DaemonSet does not enforce NetworkPolicy"}, nil
	}
	return Detection{Enforcement: EnforcementUnknown, Reason: "CNI enforcement could not be identified"}, nil
}

func groupPresent(groups []metav1.APIGroup, name string) bool {
	for _, group := range groups {
		if strings.EqualFold(group.Name, name) {
			return true
		}
	}
	return false
}

func daemonSetNamed(daemonSets []appsv1.DaemonSet, name string) bool {
	for _, daemonSet := range daemonSets {
		if daemonSet.Namespace == kubeSystemNamespace && strings.EqualFold(daemonSet.Name, name) {
			return true
		}
	}
	return false
}

var _ DiscoveryClient = discovery.DiscoveryInterface(nil)
var _ DaemonSetClient = v1.DaemonSetInterface(nil)
