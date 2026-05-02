package agentctl

import (
	"fmt"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Client wraps the Kubernetes clients needed for agentctl operations.
type Client struct {
	Dynamic   dynamic.Interface
	Kube      kubernetes.Interface
	Discovery discovery.DiscoveryInterface
}

// NewClient creates a Client from a rest.Config.
func NewClient(cfg *rest.Config) (*Client, error) {
	if cfg.UserAgent == "" {
		cfg.UserAgent = "agentctl-web"
	}

	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create dynamic client: %w", err)
	}
	kube, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes client: %w", err)
	}
	disco, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create discovery client: %w", err)
	}
	return &Client{Dynamic: dyn, Kube: kube, Discovery: disco}, nil
}

// NewInClusterClient creates a Client using in-cluster config.
func NewInClusterClient() (*Client, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("in-cluster config: %w", err)
	}
	return NewClient(cfg)
}
