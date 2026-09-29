package service

import (
	"context"
	"strings"
	"testing"

	"github.com/codeofmario/wiremap/internal/wiremap/dto"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func endpointSlice(service string, pods ...string) *discoveryv1.EndpointSlice {
	slice := &discoveryv1.EndpointSlice{ObjectMeta: metav1.ObjectMeta{
		Name: service + "-x", Namespace: testNS, Labels: map[string]string{discoveryv1.LabelServiceName: service},
	}}
	for _, pod := range pods {
		slice.Endpoints = append(slice.Endpoints, discoveryv1.Endpoint{TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: pod}})
	}
	return slice
}

func TestPodOwnerGraphsOpenStraightToTheirPods(t *testing.T) {
	pool := newTestPool(
		&appsv1.StatefulSet{ObjectMeta: objectMeta("redis", "sts"), Spec: appsv1.StatefulSetSpec{Replicas: replicas(1)}},
		&appsv1.DaemonSet{ObjectMeta: objectMeta("agent", "ds")},
		&batchv1.Job{ObjectMeta: objectMeta("migrate", "job")},
		runningPod(objectMeta("redis-0", "p1", ownerRef("StatefulSet", "redis", "sts")), "node-a", nil),
		runningPod(objectMeta("agent-x", "p2", ownerRef("DaemonSet", "agent", "ds")), "node-a", nil),
		runningPod(objectMeta("migrate-x", "p3", ownerRef("Job", "migrate", "job")), "node-b", nil),
	)
	svc := NewKubeTopologyService(pool)

	for _, c := range []struct{ group, kind, name, pod string }{
		{"apps", "StatefulSet", "redis", "redis-0"},
		{"apps", "DaemonSet", "agent", "agent-x"},
		{"batch", "Job", "migrate", "migrate-x"},
	} {
		t.Run(c.kind, func(t *testing.T) {
			g, err := svc.Object(context.Background(), "test", testNS, c.group, c.kind, c.name)
			if err != nil {
				t.Fatal(err)
			}
			root := kubeNodeID(c.kind, testNS, c.name)
			if !hasEdge(g, root, kubeNodeID("Pod", testNS, c.pod), "owns") || len(g.Nodes) != 2 {
				t.Errorf("expected %s → %s only: %+v", root, c.pod, g)
			}
			if r := nodeIDs(g)[root]; r.Drillable || r.Weight != 2 {
				t.Errorf("root should be the heavier, non-drillable node: %+v", r)
			}
		})
	}
}

func TestObjectGraphFailsForMissingObjects(t *testing.T) {
	svc := NewKubeTopologyService(newTestPool())
	for _, c := range []struct{ group, kind string }{
		{"apps", "Deployment"}, {"apps", "StatefulSet"}, {"apps", "DaemonSet"}, {"batch", "Job"}, {"batch", "CronJob"},
		{"", "Pod"}, {"", "Service"}, {"networking.k8s.io", "Ingress"}, {"", "ConfigMap"}, {"apps", "ReplicaSet"},
	} {
		if _, err := svc.Object(context.Background(), "test", testNS, c.group, c.kind, "missing"); err == nil || !strings.Contains(err.Error(), "not found") {
			t.Errorf("%s/%s: expected not found, got %v", c.group, c.kind, err)
		}
	}
	if _, err := svc.Object(context.Background(), "missing", testNS, "apps", "Deployment", "web"); err == nil {
		t.Error("unknown cluster should fail")
	}
}

func TestCustomOwnerGraphShowsPodsAndJobs(t *testing.T) {
	owner := rollout("web", "ro-web")
	pool := newTestPool(
		owner,
		&appsv1.ReplicaSet{ObjectMeta: objectMeta("web-abc", "rs", ownerRef("Rollout", "web", "ro-web")), Spec: appsv1.ReplicaSetSpec{Replicas: replicas(1)}},
		&batchv1.Job{ObjectMeta: objectMeta("web-analysis", "job", ownerRef("Rollout", "web", "ro-web"))},
		runningPod(objectMeta("web-abc-1", "p1", ownerRef("ReplicaSet", "web-abc", "rs")), "node-a", nil),
		runningPod(objectMeta("web-analysis-1", "p2", ownerRef("Job", "web-analysis", "job")), "node-a", nil),
	)

	g, err := NewKubeTopologyService(pool).Object(context.Background(), "test", testNS, "argoproj.io", "Rollout", "web")
	if err != nil {
		t.Fatal(err)
	}

	root := kubeNodeID("Rollout", testNS, "web")
	job := kubeNodeID("Job", testNS, "web-analysis")
	if !hasEdge(g, root, kubeNodeID("Pod", testNS, "web-abc-1"), "owns") || !hasEdge(g, root, job, "owns") {
		t.Errorf("custom owner should show its pods and jobs: %+v", g.Edges)
	}
	if !nodeIDs(g)[job].Drillable {
		t.Error("jobs should open their pods")
	}
	if _, ok := nodeIDs(g)[kubeNodeID("Pod", testNS, "web-analysis-1")]; ok {
		t.Error("job pods belong one level deeper")
	}
}

func TestClaimGraphShowsConsumersVolumeAndClass(t *testing.T) {
	standard := "standard"
	consumer := runningPod(objectMeta("redis-0", "p1"), "node-a", nil)
	consumer.Spec.Volumes = []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{
		PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "data"},
	}}}
	pool := newTestPool(
		&corev1.PersistentVolumeClaim{
			ObjectMeta: objectMeta("data", "pvc"),
			Spec:       corev1.PersistentVolumeClaimSpec{VolumeName: "pv-1", StorageClassName: &standard},
			Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound},
		},
		&corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{Name: "pv-1"},
			Spec: corev1.PersistentVolumeSpec{
				StorageClassName: standard,
				Capacity:         corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")},
			},
			Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeBound},
		},
		consumer,
	)

	g, err := NewKubeTopologyService(pool).Object(context.Background(), "test", testNS, "", "PersistentVolumeClaim", "data")
	if err != nil {
		t.Fatal(err)
	}

	claim := kubeNodeID("PersistentVolumeClaim", testNS, "data")
	pv := kubeNodeID("PersistentVolume", "", "pv-1")
	if !hasEdge(g, kubeNodeID("Pod", testNS, "redis-0"), claim, "uses") || !hasEdge(g, claim, pv, "bound") ||
		!hasEdge(g, pv, kubeNodeID("StorageClass", "", "standard"), "uses") {
		t.Errorf("missing pod → claim → volume → class chain: %+v", g.Edges)
	}
}

func TestPendingClaimGraphPointsAtItsClass(t *testing.T) {
	fast := "fast"
	pool := newTestPool(&corev1.PersistentVolumeClaim{
		ObjectMeta: objectMeta("scratch", "pvc"),
		Spec:       corev1.PersistentVolumeClaimSpec{StorageClassName: &fast},
		Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimPending},
	})

	g, err := NewKubeTopologyService(pool).Object(context.Background(), "test", testNS, "", "PersistentVolumeClaim", "scratch")
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(g, kubeNodeID("PersistentVolumeClaim", testNS, "scratch"), kubeNodeID("StorageClass", "", "fast"), "uses") {
		t.Errorf("pending claim should link to its class: %+v", g.Edges)
	}
}

func TestIngressGraphMarksMissingBackends(t *testing.T) {
	pool := newTestPool(&networkingv1.Ingress{
		ObjectMeta: objectMeta("shop", "ing"),
		Spec: networkingv1.IngressSpec{DefaultBackend: &networkingv1.IngressBackend{
			Service: &networkingv1.IngressServiceBackend{Name: "gone"},
		}},
	})

	g, err := NewKubeTopologyService(pool).Object(context.Background(), "test", testNS, "networking.k8s.io", "Ingress", "shop")
	if err != nil {
		t.Fatal(err)
	}
	gone := kubeNodeID("Service", testNS, "gone")
	if n := nodeIDs(g)[gone]; n.Status != statusWarning || n.Summary != "missing" || n.Drillable {
		t.Errorf("missing backend should warn: %+v", n)
	}
	if !hasEdge(g, kubeNodeID("Ingress", testNS, "shop"), gone, "routes") {
		t.Errorf("ingress should still route to the missing service: %+v", g.Edges)
	}
	if root := nodeIDs(g)[kubeNodeID("Ingress", testNS, "shop")]; root.Summary != "*" {
		t.Errorf("ingress without hosts should match any host: %+v", root)
	}
}

func TestSelectorlessServiceGraphUsesEndpointSlices(t *testing.T) {
	pool := newTestPool(
		&corev1.Service{ObjectMeta: objectMeta("legacy", "svc")},
		runningPod(objectMeta("backend-1", "p1"), "node-a", nil),
		runningPod(objectMeta("other", "p2"), "node-a", nil),
		endpointSlice("legacy", "backend-1"),
	)

	g, err := NewKubeTopologyService(pool).Object(context.Background(), "test", testNS, "", "Service", "legacy")
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(g, kubeNodeID("Service", testNS, "legacy"), kubeNodeID("Pod", testNS, "backend-1"), "selects") || len(g.Nodes) != 2 {
		t.Errorf("service should select its endpoint pod only: %+v", g)
	}
}

func TestContainerStatus(t *testing.T) {
	cases := []struct {
		name string
		ctr  dto.KubeContainerDto
		want string
	}{
		{"running and ready", dto.KubeContainerDto{State: "running", Ready: true}, statusHealthy},
		{"running init container", dto.KubeContainerDto{State: "running", Init: true}, statusHealthy},
		{"running but not ready", dto.KubeContainerDto{State: "running"}, statusWarning},
		{"completed", dto.KubeContainerDto{State: "terminated", Reason: "Completed"}, statusIdle},
		{"killed", dto.KubeContainerDto{State: "terminated", Reason: "OOMKilled"}, statusError},
		{"starting", dto.KubeContainerDto{State: "waiting", Reason: "ContainerCreating"}, statusWarning},
		{"no status yet", dto.KubeContainerDto{State: "waiting"}, statusWarning},
		{"image pull failure", dto.KubeContainerDto{State: "waiting", Reason: "ImagePullBackOff"}, statusError},
	}
	for _, c := range cases {
		if got := containerStatus(c.ctr); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestContainerNodeSummarisesStateAndSpec(t *testing.T) {
	spec := corev1.Container{
		Name: "app", Image: "app:1", Command: []string{"serve"}, Args: []string{"--port", "80"},
		Ports: []corev1.ContainerPort{{ContainerPort: 80, Protocol: corev1.ProtocolTCP}},
	}
	node := containerNode("id", testNS, dto.KubeContainerDto{Name: "app", Image: "app:1", State: "waiting", Reason: "CrashLoopBackOff", Init: true, RestartCount: 4}, spec)

	if node.Summary != "init · CrashLoopBackOff · 4 restarts" || node.Status != statusError || !node.Drillable {
		t.Errorf("unexpected node: %+v", node)
	}
	if node.Details["Ports"] != "80/TCP" || node.Details["Command"] != "serve --port 80" || node.Details["Ready"] != "false" {
		t.Errorf("unexpected details: %+v", node.Details)
	}
}
