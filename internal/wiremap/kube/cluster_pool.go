package kube

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/codeofmario/wiremap/internal/wiremap/config"
	apperrors "github.com/codeofmario/wiremap/internal/wiremap/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
)

// Cluster bundles the clients needed to talk to one Kubernetes cluster.
// Dynamic and Mapper give access to any kind the cluster serves, including CRDs.
type Cluster struct {
	Name       string
	RestConfig *rest.Config
	Clientset  kubernetes.Interface
	Metrics    metricsclient.Interface
	Dynamic    dynamic.Interface
	Mapper     meta.RESTMapper
}

type ClusterPool struct {
	clusters map[string]*Cluster
	configs  []config.ClusterConfig
	mu       sync.RWMutex
}

func NewClusterPool(settings *config.Settings) *ClusterPool {
	pool := &ClusterPool{
		clusters: make(map[string]*Cluster),
	}
	if len(settings.Clusters) == 0 {
		return pool
	}

	var connected, failed []string
	for _, cfg := range settings.Clusters {
		restConfig, contextName, err := loadRestConfig(cfg)
		if cfg.Name == "" {
			cfg.Name = contextName
		}
		pool.configs = append(pool.configs, cfg)
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", cfg.Name, err))
			continue
		}

		cluster, err := newCluster(cfg.Name, restConfig)
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", cfg.Name, err))
			continue
		}

		if err := ping(cluster); err != nil {
			failed = append(failed, fmt.Sprintf("%s (%s): %v", cfg.Name, restConfig.Host, err))
			continue
		}

		pool.clusters[cfg.Name] = cluster
		connected = append(connected, fmt.Sprintf("%s (%s)", cfg.Name, restConfig.Host))
	}

	fmt.Fprintln(os.Stderr, "Kubernetes cluster registry:")
	for _, c := range connected {
		fmt.Fprintf(os.Stderr, "  ✓ connected: %s\n", c)
	}
	for _, c := range failed {
		fmt.Fprintf(os.Stderr, "  ✗ failed:    %s\n", c)
	}

	return pool
}

func (p *ClusterPool) Get(name string) (*Cluster, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if c, ok := p.clusters[name]; ok {
		return c, nil
	}

	for _, cfg := range p.configs {
		if cfg.Name == name {
			return nil, apperrors.NotFound(fmt.Sprintf("cluster %q is configured but failed to connect at startup — check server logs", name))
		}
	}
	return nil, apperrors.NotFound(fmt.Sprintf("cluster %q is not configured", name))
}

func (p *ClusterPool) Clusters() []config.ClusterConfig {
	return p.configs
}

// Connected reports how many clusters connected at startup.
func (p *ClusterPool) Connected() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.clusters)
}

func (p *ClusterPool) IsConnected(name string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	_, ok := p.clusters[name]
	return ok
}

func loadRestConfig(cfg config.ClusterConfig) (*rest.Config, string, error) {
	if cfg.InCluster {
		restConfig, err := rest.InClusterConfig()
		return restConfig, "in-cluster", err
	}

	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if cfg.Kubeconfig != "" {
		rules.ExplicitPath = expandHome(cfg.Kubeconfig)
	}
	overrides := &clientcmd.ConfigOverrides{CurrentContext: cfg.Context}
	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)

	contextName := cfg.Context
	if contextName == "" {
		raw, err := loader.RawConfig()
		if err != nil {
			return nil, "kubernetes", err
		}
		contextName = raw.CurrentContext
	}

	restConfig, err := loader.ClientConfig()
	return restConfig, contextName, err
}

func newCluster(name string, restConfig *rest.Config) (*Cluster, error) {
	// client-go defaults (5 QPS, burst 10) suit controllers; one UI view issues dozens of reads
	restConfig.QPS = 50
	restConfig.Burst = 100
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}
	metrics, err := metricsclient.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics client: %w", err)
	}
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(clientset.Discovery()))
	return &Cluster{
		Name:       name,
		RestConfig: restConfig,
		Clientset:  clientset,
		Metrics:    metrics,
		Dynamic:    dynamicClient,
		Mapper:     mapper,
	}, nil
}

func ping(cluster *Cluster) error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	return cluster.Clientset.Discovery().RESTClient().Get().AbsPath("/version").Do(ctx).Error()
}

func expandHome(path string) string {
	if len(path) < 2 || path[:2] != "~/" {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return home + path[1:]
}

// NewStaticClusterPool builds a pool from already-connected clusters.
func NewStaticClusterPool(clusters ...*Cluster) *ClusterPool {
	pool := &ClusterPool{clusters: make(map[string]*Cluster)}
	for _, c := range clusters {
		pool.clusters[c.Name] = c
		pool.configs = append(pool.configs, config.ClusterConfig{Name: c.Name})
	}
	return pool
}
