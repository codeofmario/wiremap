package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/codeofmario/wiremap/internal/wiremap/kube"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
)

var podMetricsGVR = schema.GroupVersionResource{Group: "metrics.k8s.io", Version: "v1beta1", Resource: "pods"}

// newMetricsPool builds a cluster whose metrics-server reports the given pod usage.
func newMetricsPool(t *testing.T, usage *metricsv1beta1.PodMetrics, objects ...runtime.Object) *kube.ClusterPool {
	t.Helper()
	metrics := metricsfake.NewSimpleClientset()
	if usage != nil {
		// The fake guesses the resource "podmetricses"; the real API serves them as "pods"
		if err := metrics.Tracker().Create(podMetricsGVR, usage, usage.Namespace); err != nil {
			t.Fatal(err)
		}
	}
	return kube.NewStaticClusterPool(&kube.Cluster{Name: "test", Clientset: fake.NewSimpleClientset(objects...), Metrics: metrics})
}

func podUsage(name string, containers map[string][2]string) *metricsv1beta1.PodMetrics {
	m := &metricsv1beta1.PodMetrics{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNS},
		Timestamp:  metav1.NewTime(time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)),
	}
	for ctr, usage := range containers {
		m.Containers = append(m.Containers, metricsv1beta1.ContainerMetrics{Name: ctr, Usage: corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse(usage[0]), corev1.ResourceMemory: resource.MustParse(usage[1]),
		}})
	}
	return m
}

func limits(cpu, memory string) corev1.ResourceRequirements {
	list := corev1.ResourceList{}
	if cpu != "" {
		list[corev1.ResourceCPU] = resource.MustParse(cpu)
	}
	if memory != "" {
		list[corev1.ResourceMemory] = resource.MustParse(memory)
	}
	return corev1.ResourceRequirements{Limits: list}
}

func TestPodInspectListsAppAndInitContainers(t *testing.T) {
	pod := runningPod(objectMeta("web-a", "p1"), "node-a", map[string]string{"app": "web"})
	start := metav1.NewTime(time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC))
	pod.Status.StartTime = &start
	pod.Status.PodIP = "10.0.0.5"
	pod.Status.ContainerStatuses[0].State = corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}
	pod.Spec.InitContainers = []corev1.Container{{Name: "migrate", Image: "migrate:1"}}
	pod.Status.InitContainerStatuses = []corev1.ContainerStatus{{
		Name: "migrate", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "Completed"}},
	}}
	pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{Name: "sidecar", Image: "proxy:1"})
	pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, corev1.ContainerStatus{
		Name: "sidecar", RestartCount: 3, State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
	})

	got, err := NewKubePodService(newTestPool(pod)).Inspect(context.Background(), "test", testNS, "web-a")
	if err != nil {
		t.Fatal(err)
	}

	if got.Node != "node-a" || got.PodIP != "10.0.0.5" || got.StartTime != "2026-09-29T10:00:00Z" || got.Status != statusError || got.Labels["app"] != "web" {
		t.Errorf("unexpected pod: %+v", got)
	}
	if len(got.Containers) != 3 {
		t.Fatalf("expected 3 containers, got %+v", got.Containers)
	}
	app, sidecar, migrate := got.Containers[0], got.Containers[1], got.Containers[2]
	if app.State != "running" || !app.Ready || app.Init {
		t.Errorf("app container: %+v", app)
	}
	if sidecar.State != "waiting" || sidecar.Reason != "CrashLoopBackOff" || sidecar.RestartCount != 3 {
		t.Errorf("sidecar container: %+v", sidecar)
	}
	if migrate.State != "terminated" || migrate.Reason != "Completed" || !migrate.Init {
		t.Errorf("init container: %+v", migrate)
	}
}

func TestPodServiceReportsMissingPodsAndClusters(t *testing.T) {
	svc := NewKubePodService(newMetricsPool(t, nil))
	ctx := context.Background()

	if _, err := svc.Inspect(ctx, "test", testNS, "missing"); err == nil || !strings.Contains(err.Error(), "get pod") {
		t.Errorf("Inspect of a missing pod: %v", err)
	}
	if _, err := svc.Metrics(ctx, "test", testNS, "missing", ""); err == nil || !strings.Contains(err.Error(), "get pod") {
		t.Errorf("Metrics of a missing pod: %v", err)
	}
	for name, call := range map[string]func() error{
		"Inspect": func() error { _, err := svc.Inspect(ctx, "nope", testNS, "p"); return err },
		"Logs":    func() error { _, err := svc.Logs(ctx, "nope", testNS, "p", "app", 10); return err },
		"Metrics": func() error { _, err := svc.Metrics(ctx, "nope", testNS, "p", ""); return err },
		"Exec":    func() error { return svc.Exec(ctx, "nope", testNS, "p", "app", remotecommand.StreamOptions{}) },
	} {
		if err := call(); err == nil || !strings.Contains(err.Error(), "not configured") {
			t.Errorf("%s on an unknown cluster: %v", name, err)
		}
	}
}

func TestPodLogsStreamsFromTheAPI(t *testing.T) {
	pool := newTestPool(runningPod(objectMeta("web-a", "p1"), "node-a", nil))

	stream, err := NewKubePodService(pool).Logs(context.Background(), "test", testNS, "web-a", "app", 100)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	body, err := io.ReadAll(stream)
	if err != nil {
		t.Fatal(err)
	}
	// The fake clientset answers every log request with a fixed body
	if string(body) != "fake logs" {
		t.Errorf("unexpected log body %q", body)
	}
}

func TestPodMetricsAgainstContainerLimits(t *testing.T) {
	pod := runningPod(objectMeta("web-a", "p1"), "node-a", nil)
	pod.Spec.Containers[0].Resources = limits("500m", "256Mi")
	pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{Name: "sidecar", Resources: limits("500m", "256Mi")})
	usage := podUsage("web-a", map[string][2]string{"app": {"250m", "64Mi"}, "sidecar": {"100m", "64Mi"}})

	svc := NewKubePodService(newMetricsPool(t, usage, pod))

	stats, err := svc.Metrics(context.Background(), "test", testNS, "web-a", "app")
	if err != nil {
		t.Fatal(err)
	}
	if stats.CPUPercent != 50 || stats.MemoryPercent != 25 || stats.MemoryUsage != 64<<20 || stats.MemoryLimit != 256<<20 {
		t.Errorf("single container stats: %+v", stats)
	}
	if stats.Timestamp != "2026-09-29T12:00:00Z" {
		t.Errorf("timestamp %q", stats.Timestamp)
	}

	whole, err := svc.Metrics(context.Background(), "test", testNS, "web-a", "")
	if err != nil {
		t.Fatal(err)
	}
	if whole.CPUPercent != 35 || whole.MemoryPercent != 25 {
		t.Errorf("whole pod stats should sum usage and limits: %+v", whole)
	}
}

func TestPodMetricsWithoutLimitsUsesOneCoreAndNodeMemory(t *testing.T) {
	pod := runningPod(objectMeta("web-a", "p1"), "node-a", nil)
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-a"},
		Status:     corev1.NodeStatus{Allocatable: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")}},
	}
	usage := podUsage("web-a", map[string][2]string{"app": {"333m", "256Mi"}})

	stats, err := NewKubePodService(newMetricsPool(t, usage, pod, node)).Metrics(context.Background(), "test", testNS, "web-a", "app")
	if err != nil {
		t.Fatal(err)
	}
	if stats.CPUPercent != 33.3 || stats.MemoryLimit != 1<<30 || stats.MemoryPercent != 25 {
		t.Errorf("unexpected stats: %+v", stats)
	}
}

func TestPodMetricsWithoutMemoryReferenceLeavesPercentEmpty(t *testing.T) {
	// Unscheduled and unlimited: there's nothing to compare memory against
	pod := runningPod(objectMeta("web-a", "p1"), "", nil)
	usage := podUsage("web-a", map[string][2]string{"app": {"100m", "10Mi"}})

	stats, err := NewKubePodService(newMetricsPool(t, usage, pod)).Metrics(context.Background(), "test", testNS, "web-a", "")
	if err != nil {
		t.Fatal(err)
	}
	if stats.MemoryLimit != 0 || stats.MemoryPercent != 0 || stats.CPUPercent != 10 {
		t.Errorf("unexpected stats: %+v", stats)
	}
}

func TestPodMetricsExplainsMissingMetricsServer(t *testing.T) {
	pod := runningPod(objectMeta("web-a", "p1"), "node-a", nil)

	_, err := NewKubePodService(newMetricsPool(t, nil, pod)).Metrics(context.Background(), "test", testNS, "web-a", "")
	if err == nil || !strings.Contains(err.Error(), "metrics-server") {
		t.Errorf("expected a metrics-server hint, got %v", err)
	}
}

func TestPodLimitsSumAcrossContainers(t *testing.T) {
	containers := []corev1.Container{
		{Name: "app", Resources: limits("500m", "128Mi")},
		{Name: "sidecar", Resources: limits("250m", "64Mi")},
	}
	if cpu, mem := podLimits(containers, ""); cpu != 750 || mem != 192<<20 {
		t.Errorf("podLimits = (%d, %d), want (750, %d)", cpu, mem, 192<<20)
	}
}

func TestRoundPercentKeepsTwoDecimals(t *testing.T) {
	if got := roundPercent(12.3456); got != 12.35 {
		t.Errorf("roundPercent = %v", got)
	}
}

func TestNodeAllocatableMemoryIsZeroWhenUnknown(t *testing.T) {
	pool := newTestPool()
	c, _ := pool.Get("test")
	svc := &kubePodService{pool: pool}
	if svc.nodeAllocatableMemory(context.Background(), c, "") != 0 || svc.nodeAllocatableMemory(context.Background(), c, "missing") != 0 {
		t.Error("unscheduled pods and unreadable nodes have no memory reference")
	}
}

func TestPodExecReportsAFailedUpgrade(t *testing.T) {
	// An API server that refuses the websocket and SPDY upgrades
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "upgrade refused", http.StatusBadRequest)
	}))
	defer server.Close()

	config := &rest.Config{Host: server.URL}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	pool := kube.NewStaticClusterPool(&kube.Cluster{Name: "test", RestConfig: config, Clientset: clientset})

	err = NewKubePodService(pool).Exec(context.Background(), "test", testNS, "web-a", "app", remotecommand.StreamOptions{
		Stdin: strings.NewReader(""), Stdout: io.Discard, Tty: true,
	})
	if err == nil || !strings.Contains(err.Error(), "exec into pod") {
		t.Errorf("expected a wrapped exec error, got %v", err)
	}
}
