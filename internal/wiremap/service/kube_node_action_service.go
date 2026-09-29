package service

import (
	"context"
	"fmt"

	"github.com/codeofmario/wiremap/internal/wiremap/kube"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
)

// KubeNodeActionService takes nodes in and out of scheduling.
type KubeNodeActionService interface {
	SetUnschedulable(ctx context.Context, cluster string, node string, unschedulable bool) error
	// Drain cordons the node and evicts its pods, returning pods whose eviction
	// was refused (e.g. by a PodDisruptionBudget).
	Drain(ctx context.Context, cluster string, node string) ([]string, error)
}

type kubeNodeActionService struct {
	pool *kube.ClusterPool
}

func NewKubeNodeActionService(pool *kube.ClusterPool) KubeNodeActionService {
	return &kubeNodeActionService{pool: pool}
}

func (s *kubeNodeActionService) SetUnschedulable(ctx context.Context, clusterName string, node string, unschedulable bool) error {
	c, err := s.pool.Get(clusterName)
	if err != nil {
		return err
	}
	patch := fmt.Sprintf(`{"spec":{"unschedulable":%t}}`, unschedulable)
	if _, err := c.Clientset.CoreV1().Nodes().Patch(ctx, node, types.MergePatchType, []byte(patch), metav1.PatchOptions{}); err != nil {
		return kube.WrapError(err, "update node "+node)
	}
	return nil
}

// Drain follows `kubectl drain --ignore-daemonsets`: DaemonSet pods and static
// (mirror) pods stay, everything else goes through the Eviction API so
// PodDisruptionBudgets are respected.
func (s *kubeNodeActionService) Drain(ctx context.Context, clusterName string, node string) ([]string, error) {
	if err := s.SetUnschedulable(ctx, clusterName, node, true); err != nil {
		return nil, err
	}
	c, err := s.pool.Get(clusterName)
	if err != nil {
		return nil, err
	}

	pods, err := c.Clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("spec.nodeName", node).String(),
	})
	if err != nil {
		return nil, kube.WrapError(err, "list pods on node")
	}

	var refused []string
	for i := range pods.Items {
		pod := &pods.Items[i]
		if !evictable(pod) {
			continue
		}
		eviction := &policyv1.Eviction{ObjectMeta: metav1.ObjectMeta{Name: pod.Name, Namespace: pod.Namespace}}
		if err := c.Clientset.CoreV1().Pods(pod.Namespace).EvictV1(ctx, eviction); err != nil {
			refused = append(refused, fmt.Sprintf("%s/%s: %s", pod.Namespace, pod.Name, err))
		}
	}
	return refused, nil
}

func evictable(pod *corev1.Pod) bool {
	if _, mirror := pod.Annotations[corev1.MirrorPodAnnotationKey]; mirror {
		return false
	}
	if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
		return false
	}
	for _, ref := range pod.OwnerReferences {
		if ref.Kind == "DaemonSet" {
			return false
		}
	}
	return true
}
