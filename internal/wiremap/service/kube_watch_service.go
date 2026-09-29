package service

import (
	"context"
	"time"

	"github.com/codeofmario/wiremap/internal/wiremap/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
)

const (
	// Bursts of changes (a rollout touches many objects) collapse into one notification
	watchDebounce = time.Second
	watchRetry    = 5 * time.Second
)

type watchKind struct {
	group string
	kind  string
}

// Kinds drawn at namespace and workload level
var namespaceWatchKinds = []watchKind{
	{"", "Pod"}, {"", "Service"}, {"", "ConfigMap"}, {"", "Secret"}, {"", "PersistentVolumeClaim"}, {"", "ServiceAccount"},
	{"apps", "Deployment"}, {"apps", "StatefulSet"}, {"apps", "DaemonSet"}, {"apps", "ReplicaSet"},
	{"batch", "Job"}, {"batch", "CronJob"},
	{"networking.k8s.io", "Ingress"}, {"networking.k8s.io", "NetworkPolicy"},
	{"autoscaling", "HorizontalPodAutoscaler"}, {"policy", "PodDisruptionBudget"},
	{"rbac.authorization.k8s.io", "RoleBinding"},
	{gatewayGroup, "Gateway"}, {gatewayGroup, "HTTPRoute"}, {gatewayGroup, "GRPCRoute"},
	{gatewayGroup, "TLSRoute"}, {gatewayGroup, "TCPRoute"}, {gatewayGroup, "UDPRoute"},
}

// Kinds drawn by the cluster-level lenses
var clusterWatchKinds = []watchKind{
	{"", "Namespace"}, {"", "Node"}, {"", "Pod"}, {"", "PersistentVolume"}, {"", "PersistentVolumeClaim"},
	{"rbac.authorization.k8s.io", "ClusterRoleBinding"}, {"rbac.authorization.k8s.io", "RoleBinding"},
}

// KubeWatchService reports changes to the objects a canvas level shows, so the
// UI can refresh on change instead of polling.
type KubeWatchService interface {
	// Watch calls onChange after objects in scope change, until ctx is cancelled.
	// An empty namespace watches the cluster-level lenses.
	Watch(ctx context.Context, cluster string, namespace string, onChange func()) error
}

type kubeWatchService struct {
	pool *kube.ClusterPool
}

func NewKubeWatchService(pool *kube.ClusterPool) KubeWatchService {
	return &kubeWatchService{pool: pool}
}

func (s *kubeWatchService) Watch(ctx context.Context, clusterName string, namespace string, onChange func()) error {
	c, err := s.pool.Get(clusterName)
	if err != nil {
		return err
	}

	kinds := namespaceWatchKinds
	if namespace == "" {
		kinds = clusterWatchKinds
	}

	notify := debounce(ctx, watchDebounce, onChange)
	for _, k := range kinds {
		mapping, err := c.ResourceFor(k.group, k.kind)
		if err != nil {
			continue // e.g. Gateway API not installed
		}
		var client dynamic.ResourceInterface = c.Dynamic.Resource(mapping.GVR)
		if mapping.Namespaced {
			client = c.Dynamic.Resource(mapping.GVR).Namespace(namespace)
		}
		go watchResource(ctx, client, notify)
	}
	return nil
}

// watchResource streams changes from the current resourceVersion onward,
// reconnecting when the API server ends the watch (it does so every few minutes).
func watchResource(ctx context.Context, client dynamic.ResourceInterface, notify func()) {
	for ctx.Err() == nil {
		list, err := client.List(ctx, metav1.ListOptions{Limit: 1})
		if err != nil {
			if kube.IsOptional(err) {
				return // not allowed to see this kind; nothing to watch
			}
			sleep(ctx, watchRetry)
			continue
		}

		w, err := client.Watch(ctx, metav1.ListOptions{ResourceVersion: list.GetResourceVersion(), AllowWatchBookmarks: true})
		if err != nil {
			sleep(ctx, watchRetry)
			continue
		}
		for event := range w.ResultChan() {
			if event.Type == watch.Error {
				break // resourceVersion too old; relist
			}
			if event.Type != watch.Bookmark {
				notify()
			}
		}
		w.Stop()
	}
}

// debounce returns a trigger that calls fn once, delay after the first of a burst of triggers.
func debounce(ctx context.Context, delay time.Duration, fn func()) func() {
	pending := make(chan struct{}, 1)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-pending:
			}
			sleep(ctx, delay)
			if ctx.Err() != nil {
				return
			}
			select {
			case <-pending:
			default:
			}
			fn()
		}
	}()
	return func() {
		select {
		case pending <- struct{}{}:
		default:
		}
	}
}

func sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
