package service

import (
	"context"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/codeofmario/wiremap/internal/wiremap/dto"
	apperrors "github.com/codeofmario/wiremap/internal/wiremap/errors"
	"github.com/codeofmario/wiremap/internal/wiremap/kube"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

// Prefer bash when the image has it, like `kubectl exec -it ... -- bash`
var execShell = []string{"/bin/sh", "-c", "if command -v bash >/dev/null 2>&1; then exec bash; else exec sh; fi"}

type KubePodService interface {
	Inspect(ctx context.Context, cluster string, namespace string, name string) (*dto.KubePodDto, error)
	Logs(ctx context.Context, cluster string, namespace string, name string, container string, tail int64) (io.ReadCloser, error)
	Metrics(ctx context.Context, cluster string, namespace string, name string, container string) (*dto.StatsDto, error)
	Exec(ctx context.Context, cluster string, namespace string, name string, container string, streams remotecommand.StreamOptions) error
}

type kubePodService struct {
	pool *kube.ClusterPool
}

func NewKubePodService(pool *kube.ClusterPool) KubePodService {
	return &kubePodService{pool: pool}
}

func (s *kubePodService) Inspect(ctx context.Context, clusterName string, namespace string, name string) (*dto.KubePodDto, error) {
	c, err := s.pool.Get(clusterName)
	if err != nil {
		return nil, err
	}

	pod, err := c.Clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, kube.WrapError(err, "get pod")
	}

	status, _ := podStatus(pod)
	result := &dto.KubePodDto{
		Name:       pod.Name,
		Namespace:  pod.Namespace,
		Node:       pod.Spec.NodeName,
		Phase:      string(pod.Status.Phase),
		Status:     status,
		PodIP:      pod.Status.PodIP,
		HostIP:     pod.Status.HostIP,
		Labels:     pod.Labels,
		Containers: []dto.KubeContainerDto{},
	}
	if pod.Status.StartTime != nil {
		result.StartTime = pod.Status.StartTime.Format(time.RFC3339)
	}

	result.Containers = append(result.Containers, containerDtos(pod.Spec.Containers, pod.Status.ContainerStatuses, false)...)
	result.Containers = append(result.Containers, containerDtos(pod.Spec.InitContainers, pod.Status.InitContainerStatuses, true)...)
	return result, nil
}

func containerDtos(specs []corev1.Container, statuses []corev1.ContainerStatus, init bool) []dto.KubeContainerDto {
	byName := make(map[string]corev1.ContainerStatus, len(statuses))
	for _, st := range statuses {
		byName[st.Name] = st
	}

	result := make([]dto.KubeContainerDto, 0, len(specs))
	for _, spec := range specs {
		st := byName[spec.Name]
		d := dto.KubeContainerDto{
			Name:         spec.Name,
			Image:        spec.Image,
			Ready:        st.Ready,
			RestartCount: st.RestartCount,
			State:        "waiting",
			Init:         init,
		}
		switch {
		case st.State.Running != nil:
			d.State = "running"
		case st.State.Terminated != nil:
			d.State, d.Reason = "terminated", st.State.Terminated.Reason
		case st.State.Waiting != nil:
			d.Reason = st.State.Waiting.Reason
		}
		result = append(result, d)
	}
	return result
}

func (s *kubePodService) Logs(ctx context.Context, clusterName string, namespace string, name string, container string, tail int64) (io.ReadCloser, error) {
	c, err := s.pool.Get(clusterName)
	if err != nil {
		return nil, err
	}

	stream, err := c.Clientset.CoreV1().Pods(namespace).GetLogs(name, &corev1.PodLogOptions{
		Container: container,
		Follow:    true,
		TailLines: &tail,
	}).Stream(ctx)
	if err != nil {
		return nil, kube.WrapError(err, "stream pod logs")
	}
	return stream, nil
}

// Metrics reports pod usage from metrics-server. Without resource limits, CPU is
// relative to one core and memory to the node's allocatable memory.
func (s *kubePodService) Metrics(ctx context.Context, clusterName string, namespace string, name string, container string) (*dto.StatsDto, error) {
	c, err := s.pool.Get(clusterName)
	if err != nil {
		return nil, err
	}

	pod, err := c.Clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, kube.WrapError(err, "get pod")
	}

	metrics, err := c.Metrics.MetricsV1beta1().PodMetricses(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, apperrors.NotFound("pod metrics unavailable — is metrics-server installed and has it scraped this pod yet?")
		}
		return nil, kube.WrapError(err, "get pod metrics")
	}

	var cpuUsage, memUsage int64
	for _, cm := range metrics.Containers {
		if container != "" && cm.Name != container {
			continue
		}
		cpuUsage += cm.Usage.Cpu().MilliValue()
		memUsage += cm.Usage.Memory().Value()
	}

	cpuLimit, memLimit := podLimits(pod.Spec.Containers, container)
	if cpuLimit == 0 {
		cpuLimit = 1000
	}
	if memLimit == 0 {
		memLimit = s.nodeAllocatableMemory(ctx, c, pod.Spec.NodeName)
	}

	stats := &dto.StatsDto{
		CPUPercent:  roundPercent(float64(cpuUsage) / float64(cpuLimit) * 100),
		MemoryUsage: uint64(memUsage),
		MemoryLimit: uint64(memLimit),
		Timestamp:   metrics.Timestamp.Format(time.RFC3339),
	}
	if memLimit > 0 {
		stats.MemoryPercent = roundPercent(float64(memUsage) / float64(memLimit) * 100)
	}
	return stats, nil
}

// podLimits sums CPU (millicores) and memory (bytes) limits, returning 0 for a
// resource when any matching container is unlimited.
func podLimits(containers []corev1.Container, container string) (int64, int64) {
	var cpu, mem int64
	cpuUnlimited, memUnlimited := false, false
	for _, ctr := range containers {
		if container != "" && ctr.Name != container {
			continue
		}
		cpuQty, hasCPU := ctr.Resources.Limits[corev1.ResourceCPU]
		memQty, hasMem := ctr.Resources.Limits[corev1.ResourceMemory]
		cpuUnlimited = cpuUnlimited || !hasCPU
		memUnlimited = memUnlimited || !hasMem
		cpu += cpuQty.MilliValue()
		mem += memQty.Value()
	}
	if cpuUnlimited {
		cpu = 0
	}
	if memUnlimited {
		mem = 0
	}
	return cpu, mem
}

func (s *kubePodService) nodeAllocatableMemory(ctx context.Context, c *kube.Cluster, nodeName string) int64 {
	if nodeName == "" {
		return 0
	}
	node, err := c.Clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return 0 // RBAC may forbid reading nodes; the limit is only informative
	}
	mem := node.Status.Allocatable[corev1.ResourceMemory]
	return mem.Value()
}

func roundPercent(v float64) float64 {
	return math.Round(v*100) / 100
}

func (s *kubePodService) Exec(ctx context.Context, clusterName string, namespace string, name string, container string, streams remotecommand.StreamOptions) error {
	c, err := s.pool.Get(clusterName)
	if err != nil {
		return err
	}

	req := c.Clientset.CoreV1().RESTClient().Post().
		Resource("pods").Namespace(namespace).Name(name).SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   execShell,
			Stdin:     true,
			Stdout:    true,
			TTY:       true,
		}, scheme.ParameterCodec)

	wsExec, err := remotecommand.NewWebSocketExecutor(c.RestConfig, "GET", req.URL().String())
	if err != nil {
		return apperrors.Internal(fmt.Sprintf("failed to create exec: %s", err))
	}
	spdyExec, err := remotecommand.NewSPDYExecutor(c.RestConfig, "POST", req.URL())
	if err != nil {
		return apperrors.Internal(fmt.Sprintf("failed to create exec: %s", err))
	}
	// Same fallback as kubectl: websockets first, SPDY for older API servers
	executor, err := remotecommand.NewFallbackExecutor(wsExec, spdyExec, func(err error) bool {
		return httpstream.IsUpgradeFailure(err) || httpstream.IsHTTPSProxyError(err)
	})
	if err != nil {
		return apperrors.Internal(fmt.Sprintf("failed to create exec: %s", err))
	}

	if err := executor.StreamWithContext(ctx, streams); err != nil {
		return kube.WrapError(err, "exec into pod")
	}
	return nil
}
