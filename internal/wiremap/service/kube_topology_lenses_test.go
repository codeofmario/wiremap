package service

import (
	"context"
	"testing"

	"github.com/codeofmario/wiremap/internal/wiremap/kube"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestClusterServiceListsConfiguredClusters(t *testing.T) {
	clusters := NewKubeClusterService(kube.NewStaticClusterPool(&kube.Cluster{Name: "prod"}, &kube.Cluster{Name: "staging"})).List()
	if len(clusters) != 2 || clusters[0].Name != "prod" || !clusters[0].Connected || clusters[1].Name != "staging" {
		t.Errorf("unexpected clusters: %+v", clusters)
	}
	if got := NewKubeClusterService(newTestPool()).List(); len(got) != 1 {
		t.Errorf("expected the test cluster only, got %+v", got)
	}
}

func testNode(name string, labels map[string]string, unschedulable bool) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		Spec:       corev1.NodeSpec{Unschedulable: unschedulable},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
			NodeInfo:   corev1.NodeSystemInfo{KubeletVersion: "v1.34.0", OSImage: "Debian"},
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("4"), corev1.ResourceMemory: resource.MustParse("8Gi"),
			},
		},
	}
}

func TestNodesLensCountsPodsPerNode(t *testing.T) {
	pool := newTestPool(
		testNode("cp", map[string]string{"node-role.kubernetes.io/control-plane": ""}, false),
		testNode("worker", nil, true),
		runningPod(objectMeta("a", "p1"), "worker", nil),
		runningPod(objectMeta("b", "p2"), "worker", nil),
	)

	g, err := NewKubeTopologyService(pool).Cluster(context.Background(), "test", LensNodes)
	if err != nil {
		t.Fatal(err)
	}

	nodes := nodeIDs(g)
	cp := nodes[kubeNodeID("Node", "", "cp")]
	worker := nodes[kubeNodeID("Node", "", "worker")]
	if cp.Status != statusHealthy || cp.Details["Roles"] != "control-plane" || cp.Summary != "0 pods · v1.34.0" || !cp.Drillable {
		t.Errorf("control plane node: %+v", cp)
	}
	if worker.Status != statusWarning || worker.Weight != 2 || worker.Details["Roles"] != "worker" || worker.Details["CPU"] != "4" {
		t.Errorf("cordoned worker: %+v", worker)
	}
}

func TestNodeRolesAreSortedAndDefaultToWorker(t *testing.T) {
	node := testNode("n", map[string]string{
		"node-role.kubernetes.io/master":        "",
		"node-role.kubernetes.io/control-plane": "",
		"kubernetes.io/hostname":                "n",
	}, false)
	if got := nodeRoles(node); got != "control-plane, master" {
		t.Errorf("nodeRoles = %q", got)
	}
	if got := nodeRoles(testNode("n", nil, false)); got != "worker" {
		t.Errorf("nodeRoles without labels = %q", got)
	}
}

func TestNodeLevelGroupsItsPodsByNamespace(t *testing.T) {
	other := runningPod(objectMeta("dns", "p2"), "worker", nil)
	other.Namespace = "kube-system"
	// The fake clientset ignores field selectors, so only pods on this node are seeded
	pool := newTestPool(runningPod(objectMeta("web", "p1"), "worker", nil), other)

	g, err := NewKubeTopologyService(pool).Node(context.Background(), "test", "worker")
	if err != nil {
		t.Fatal(err)
	}

	nodes := nodeIDs(g)
	if len(g.Nodes) != 2 || len(g.Groups) != 2 {
		t.Fatalf("expected two pods in two namespace groups, got %+v", g)
	}
	if web := nodes[kubeNodeID("Pod", testNS, "web")]; web.Group != kubeNodeID("Namespace", "", testNS) || !web.Drillable {
		t.Errorf("pod should be grouped by namespace and drillable: %+v", web)
	}
	if _, err := NewKubeTopologyService(pool).Node(context.Background(), "missing", "worker"); err == nil {
		t.Error("unknown cluster should fail")
	}
}

func TestStorageLensDrawsClaimsVolumesAndClasses(t *testing.T) {
	standard := "standard"
	fast := "fast"
	pool := newTestPool(
		&storagev1.StorageClass{
			ObjectMeta:  metav1.ObjectMeta{Name: "standard", Annotations: map[string]string{"storageclass.kubernetes.io/is-default-class": "true"}},
			Provisioner: "rancher.io/local-path",
		},
		&corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{Name: "pv-1"},
			Spec: corev1.PersistentVolumeSpec{
				StorageClassName: standard,
				Capacity:         corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")},
			},
			Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeBound},
		},
		&corev1.PersistentVolumeClaim{
			ObjectMeta: objectMeta("data", "pvc-1"),
			Spec:       corev1.PersistentVolumeClaimSpec{VolumeName: "pv-1", StorageClassName: &standard},
			Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound},
		},
		&corev1.PersistentVolumeClaim{
			ObjectMeta: objectMeta("waiting", "pvc-2"),
			Spec:       corev1.PersistentVolumeClaimSpec{StorageClassName: &fast},
			Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimPending},
		},
	)

	g, err := NewKubeTopologyService(pool).Cluster(context.Background(), "test", LensStorage)
	if err != nil {
		t.Fatal(err)
	}

	nodes := nodeIDs(g)
	data := kubeNodeID("PersistentVolumeClaim", testNS, "data")
	waiting := kubeNodeID("PersistentVolumeClaim", testNS, "waiting")
	pv := kubeNodeID("PersistentVolume", "", "pv-1")
	if !hasEdge(g, data, pv, "bound") || !hasEdge(g, pv, kubeNodeID("StorageClass", "", "standard"), "uses") {
		t.Errorf("missing bound claim chain: %+v", g.Edges)
	}
	if !hasEdge(g, waiting, kubeNodeID("StorageClass", "", "fast"), "uses") {
		t.Errorf("pending claim should point at its class: %+v", g.Edges)
	}
	if sc := nodes[kubeNodeID("StorageClass", "", "standard")]; sc.Details["Default"] != "yes" || sc.Summary != "rancher.io/local-path" {
		t.Errorf("storage class: %+v", sc)
	}
	if claim := nodes[data]; claim.Summary != testNS+" · Bound" || !claim.Drillable {
		t.Errorf("claim: %+v", claim)
	}
	if nodes[waiting].Status != statusWarning {
		t.Errorf("pending claim should warn: %+v", nodes[waiting])
	}
}
