package netpolicy

import (
	"context"
	"errors"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDetectCNIEnforcement(t *testing.T) {
	tests := []struct {
		name       string
		groups     []metav1.APIGroup
		daemonSets []string
		want       Enforcement
	}{
		{name: "Cilium API group", groups: []metav1.APIGroup{{Name: "cilium.io"}}, want: EnforcementEnforcing},
		{name: "Calico API group", groups: []metav1.APIGroup{{Name: "crd.projectcalico.org"}}, want: EnforcementEnforcing},
		{name: "Antrea DaemonSet", daemonSets: []string{"antrea-agent"}, want: EnforcementEnforcing},
		{name: "Weave DaemonSet", daemonSets: []string{"weave-net"}, want: EnforcementEnforcing},
		{name: "Calico overrides Flannel", daemonSets: []string{"kube-flannel-ds", "calico-node"}, want: EnforcementEnforcing},
		{name: "kindnet DaemonSet", daemonSets: []string{"kindnet"}, want: EnforcementKnownNonEnforcing},
		{name: "Flannel DaemonSet", daemonSets: []string{"kube-flannel-ds"}, want: EnforcementKnownNonEnforcing},
		{name: "unknown CNI", want: EnforcementUnknown},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			daemonSets := fakeDaemonSets(test.daemonSets...)
			detection, err := DetectCNIEnforcement(context.Background(), fakeDiscovery{groups: test.groups}, daemonSets)
			if err != nil {
				t.Fatalf("DetectCNIEnforcement: %v", err)
			}
			if detection.Enforcement != test.want {
				t.Fatalf("enforcement = %q, want %q", detection.Enforcement, test.want)
			}
		})
	}
}

func TestDetectCNIEnforcementReturnsClientErrors(t *testing.T) {
	_, err := DetectCNIEnforcement(context.Background(), fakeDiscovery{err: errCNIClient}, fakeDaemonSets())
	if !errors.Is(err, errCNIClient) {
		t.Fatalf("discovery error = %v, want %v", err, errCNIClient)
	}
	_, err = DetectCNIEnforcement(context.Background(), fakeDiscovery{}, failingDaemonSets{})
	if !errors.Is(err, errCNIClient) {
		t.Fatalf("DaemonSet error = %v, want %v", err, errCNIClient)
	}
	_, err = DetectCNIEnforcement(context.Background(), nilDiscovery{}, fakeDaemonSets())
	if err == nil {
		t.Fatal("nil discovery response was accepted")
	}
	_, err = DetectCNIEnforcement(context.Background(), fakeDiscovery{}, nilDaemonSets{})
	if err == nil {
		t.Fatal("nil DaemonSet response was accepted")
	}
}

type fakeDiscovery struct {
	groups []metav1.APIGroup
	err    error
}

func (d fakeDiscovery) ServerGroups() (*metav1.APIGroupList, error) {
	if d.err != nil {
		return nil, d.err
	}
	return &metav1.APIGroupList{Groups: d.groups}, nil
}

type daemonSetListClient struct {
	items []appsv1.DaemonSet
}

func (c daemonSetListClient) List(context.Context, metav1.ListOptions) (*appsv1.DaemonSetList, error) {
	return &appsv1.DaemonSetList{Items: c.items}, nil
}

func fakeDaemonSets(names ...string) daemonSetListClient {
	items := make([]appsv1.DaemonSet, 0, len(names))
	for _, name := range names {
		items = append(items, appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: kubeSystemNamespace}})
	}
	return daemonSetListClient{items: items}
}

type failingDaemonSets struct{}

func (failingDaemonSets) List(context.Context, metav1.ListOptions) (*appsv1.DaemonSetList, error) {
	return nil, errCNIClient
}

var errCNIClient = errors.New("kubernetes client unavailable")

type nilDiscovery struct{}

func (nilDiscovery) ServerGroups() (*metav1.APIGroupList, error) {
	return nil, nil
}

type nilDaemonSets struct{}

func (nilDaemonSets) List(context.Context, metav1.ListOptions) (*appsv1.DaemonSetList, error) {
	return nil, nil
}
