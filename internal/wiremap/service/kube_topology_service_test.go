package service

import (
	"context"
	"strings"
	"testing"

	"github.com/codeofmario/wiremap/internal/wiremap/dto"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const testNS = "shop"

func objectMeta(name string, uid string, owners ...metav1.OwnerReference) metav1.ObjectMeta {
	return metav1.ObjectMeta{Name: name, Namespace: testNS, UID: types.UID(uid), OwnerReferences: owners}
}

func ownerRef(kind, name, uid string) metav1.OwnerReference {
	return metav1.OwnerReference{Kind: kind, Name: name, UID: types.UID(uid)}
}

func replicas(n int32) *int32 { return &n }

func runningPod(meta metav1.ObjectMeta, node string, labels map[string]string) *corev1.Pod {
	meta.Labels = labels
	return &corev1.Pod{
		ObjectMeta: meta,
		Spec:       corev1.PodSpec{NodeName: node, Containers: []corev1.Container{{Name: "app", Image: "app:1"}}},
		Status: corev1.PodStatus{
			Phase:             corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{Name: "app", Ready: true}},
		},
	}
}

func webDeployment() *appsv1.Deployment {
	labels := map[string]string{"app": "web"}
	return &appsv1.Deployment{
		ObjectMeta: objectMeta("web", "dep-web"),
		Spec: appsv1.DeploymentSpec{
			Replicas: replicas(2),
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:    "app",
						Image:   "web:1",
						EnvFrom: []corev1.EnvFromSource{{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "web-config"}}}},
					}},
					Volumes: []corev1.Volume{{
						Name:         "data",
						VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "web-data"}},
					}},
				},
			},
		},
		Status: appsv1.DeploymentStatus{ReadyReplicas: 1},
	}
}

func nodeIDs(g *dto.KubeGraphDto) map[string]dto.KubeGraphNodeDto {
	m := map[string]dto.KubeGraphNodeDto{}
	for _, n := range g.Nodes {
		m[n.ID] = n
	}
	return m
}

func hasEdge(g *dto.KubeGraphDto, source, target, kind string) bool {
	for _, e := range g.Edges {
		if e.Source == source && e.Target == target && e.Kind == kind {
			return true
		}
	}
	return false
}

func TestNamespaceGraphLinksIngressServiceWorkloadAndConfig(t *testing.T) {
	pathType := networkingv1.PathTypePrefix
	pool := newTestPool(
		webDeployment(),
		&corev1.Service{
			ObjectMeta: objectMeta("web", "svc-web"),
			Spec:       corev1.ServiceSpec{Selector: map[string]string{"app": "web"}, Type: corev1.ServiceTypeClusterIP},
		},
		&networkingv1.Ingress{
			ObjectMeta: objectMeta("web", "ing-web"),
			Spec: networkingv1.IngressSpec{Rules: []networkingv1.IngressRule{{
				Host: "shop.example.com",
				IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{Path: "/", PathType: &pathType, Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{Name: "web"}}},
						{Path: "/api", PathType: &pathType, Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{Name: "web"}}},
					},
				}},
			}}},
		},
		&corev1.PersistentVolumeClaim{
			ObjectMeta: objectMeta("web-data", "pvc-web"),
			Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound},
		},
		&batchv1.CronJob{ObjectMeta: objectMeta("backup", "cj-backup"), Spec: batchv1.CronJobSpec{Schedule: "0 * * * *"}},
		&batchv1.Job{ObjectMeta: objectMeta("backup-123", "job-backup", ownerRef("CronJob", "backup", "cj-backup"))},
		runningPod(objectMeta("debug", "pod-debug"), "node-a", nil),
		&appsv1.ReplicaSet{ObjectMeta: objectMeta("web-1", "rs-web", ownerRef("Deployment", "web", "dep-web"))},
		runningPod(objectMeta("web-abc", "pod-web", ownerRef("ReplicaSet", "web-1", "rs-web")), "node-a", map[string]string{"app": "web"}),
	)

	g, err := NewKubeTopologyService(pool).Namespace(context.Background(), "test", testNS)
	if err != nil {
		t.Fatal(err)
	}

	nodes := nodeIDs(g)
	deploy := kubeNodeID("Deployment", testNS, "web")
	svc := kubeNodeID("Service", testNS, "web")
	ing := kubeNodeID("Ingress", testNS, "web")
	cm := kubeNodeID("ConfigMap", testNS, "web-config")
	pvc := kubeNodeID("PersistentVolumeClaim", testNS, "web-data")

	for _, id := range []string{deploy, svc, ing, cm, pvc, kubeNodeID("CronJob", testNS, "backup"), kubeNodeID("Pod", testNS, "debug")} {
		if _, ok := nodes[id]; !ok {
			t.Errorf("missing node %s", id)
		}
	}
	for _, id := range []string{kubeNodeID("Job", testNS, "backup-123"), kubeNodeID("Pod", testNS, "web-abc")} {
		if _, ok := nodes[id]; ok {
			t.Errorf("owned object %s should only appear when drilling down", id)
		}
	}

	if !hasEdge(g, ing, svc, "routes") || !hasEdge(g, svc, deploy, "selects") ||
		!hasEdge(g, deploy, cm, "uses") || !hasEdge(g, deploy, pvc, "uses") {
		t.Errorf("missing expected edges: %+v", g.Edges)
	}
	if len(g.Edges) != 4 {
		t.Errorf("expected 4 deduplicated edges, got %d: %+v", len(g.Edges), g.Edges)
	}

	if d := nodes[deploy]; d.Status != statusWarning || d.Summary != "1/2 ready" || !d.Drillable {
		t.Errorf("unexpected deployment node: %+v", d)
	}
	if p := nodes[pvc]; p.Status != statusHealthy {
		t.Errorf("bound PVC should be healthy: %+v", p)
	}
}

func TestDeploymentGraphOpensStraightToPodsGroupedByNode(t *testing.T) {
	deployOwner := ownerRef("Deployment", "web", "dep-web")
	pool := newTestPool(
		webDeployment(),
		&appsv1.ReplicaSet{ObjectMeta: objectMeta("web-2", "rs-new", deployOwner), Spec: appsv1.ReplicaSetSpec{Replicas: replicas(2)}},
		runningPod(objectMeta("web-2-a", "p1", ownerRef("ReplicaSet", "web-2", "rs-new")), "node-a", nil),
		runningPod(objectMeta("web-2-b", "p2", ownerRef("ReplicaSet", "web-2", "rs-new")), "", nil),
		runningPod(objectMeta("other", "p3", ownerRef("ReplicaSet", "other", "rs-other")), "node-a", nil),
	)

	g, err := NewKubeTopologyService(pool).Object(context.Background(), "test", testNS, "apps", "Deployment", "web")
	if err != nil {
		t.Fatal(err)
	}

	nodes := nodeIDs(g)
	deployment := kubeNodeID("Deployment", testNS, "web")
	if _, ok := nodes[kubeNodeID("ReplicaSet", testNS, "web-2")]; ok {
		t.Error("replicasets should be skipped")
	}
	if _, ok := nodes[kubeNodeID("Pod", testNS, "other")]; ok {
		t.Error("pods of other owners should be excluded")
	}
	if !hasEdge(g, deployment, kubeNodeID("Pod", testNS, "web-2-a"), "owns") {
		t.Errorf("missing deployment → pod edge: %+v", g.Edges)
	}
	if !nodes[kubeNodeID("Pod", testNS, "web-2-a")].Drillable {
		t.Error("pods should open their containers")
	}

	if got := nodes[kubeNodeID("Pod", testNS, "web-2-a")].Group; got != kubeNodeID("Node", "", "node-a") {
		t.Errorf("pod grouped under %q", got)
	}
	if got := nodes[kubeNodeID("Pod", testNS, "web-2-b")].Group; got != kubeNodeID("Node", "", unscheduledGroup) {
		t.Errorf("unscheduled pod grouped under %q", got)
	}
	if len(g.Groups) != 2 {
		t.Errorf("expected 2 node groups, got %+v", g.Groups)
	}
}

func TestCronJobGraphShowsJobsOnly(t *testing.T) {
	pool := newTestPool(
		&batchv1.CronJob{ObjectMeta: objectMeta("report", "cj"), Spec: batchv1.CronJobSpec{Schedule: "@daily"}},
		&batchv1.Job{ObjectMeta: objectMeta("report-1", "job-1", ownerRef("CronJob", "report", "cj"))},
		runningPod(objectMeta("report-1-a", "p1", ownerRef("Job", "report-1", "job-1")), "node-a", nil),
	)

	g, err := NewKubeTopologyService(pool).Object(context.Background(), "test", testNS, "batch", "CronJob", "report")
	if err != nil {
		t.Fatal(err)
	}

	job := kubeNodeID("Job", testNS, "report-1")
	if !hasEdge(g, kubeNodeID("CronJob", testNS, "report"), job, "owns") || !nodeIDs(g)[job].Drillable {
		t.Errorf("cronjob should open to drillable jobs: %+v", g)
	}
	if _, ok := nodeIDs(g)[kubeNodeID("Pod", testNS, "report-1-a")]; ok {
		t.Error("job pods belong one level deeper")
	}
}

func TestClusterGraphAggregatesPodHealthPerNamespace(t *testing.T) {
	crashing := runningPod(objectMeta("broken", "p2"), "node-a", nil)
	crashing.Status.ContainerStatuses[0] = corev1.ContainerStatus{
		Name:  "app",
		State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
	}

	pool := newTestPool(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNS}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "empty"}},
		runningPod(objectMeta("ok", "p1"), "node-a", nil),
		crashing,
		webDeployment(),
	)

	g, err := NewKubeTopologyService(pool).Cluster(context.Background(), "test", LensNamespaces)
	if err != nil {
		t.Fatal(err)
	}

	nodes := nodeIDs(g)
	shop := nodes[kubeNodeID("Namespace", "", testNS)]
	if shop.Status != statusError || shop.Weight != 2 || shop.Summary != "2 pods · 1 workloads" {
		t.Errorf("unexpected namespace node: %+v", shop)
	}
	if empty := nodes[kubeNodeID("Namespace", "", "empty")]; empty.Status != statusIdle {
		t.Errorf("namespace without pods should be idle: %+v", empty)
	}
}

func TestUnknownClusterIsNotFound(t *testing.T) {
	_, err := NewKubeTopologyService(newTestPool()).Cluster(context.Background(), "missing", LensNamespaces)
	if err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("expected not found, got %v", err)
	}
}
