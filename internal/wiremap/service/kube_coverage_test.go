package service

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func labelledDeployment(name string, labels map[string]string, account string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: objectMeta(name, "dep-"+name),
		Spec: appsv1.DeploymentSpec{
			Replicas: replicas(1),
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec:       corev1.PodSpec{ServiceAccountName: account, Containers: []corev1.Container{{Name: "app", Image: name}}},
			},
		},
	}
}

func TestNamespaceGraphDrawsScalingPoliciesAndAccess(t *testing.T) {
	web := map[string]string{"app": "web"}
	api := map[string]string{"app": "api"}
	minAvailable := intstr.FromInt32(1)
	cpuTarget := int32(80)

	pool := newTestPool(
		labelledDeployment("web", web, "web-sa"),
		labelledDeployment("api", api, ""),
		&autoscalingv2.HorizontalPodAutoscaler{
			ObjectMeta: objectMeta("web", "hpa"),
			Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
				ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{Kind: "Deployment", Name: "web"},
				MaxReplicas:    5,
				Metrics: []autoscalingv2.MetricSpec{{Type: autoscalingv2.ResourceMetricSourceType, Resource: &autoscalingv2.ResourceMetricSource{
					Name: corev1.ResourceCPU, Target: autoscalingv2.MetricTarget{AverageUtilization: &cpuTarget},
				}}},
			},
		},
		&policyv1.PodDisruptionBudget{
			ObjectMeta: objectMeta("web", "pdb"),
			Spec:       policyv1.PodDisruptionBudgetSpec{MinAvailable: &minAvailable, Selector: &metav1.LabelSelector{MatchLabels: web}},
		},
		&networkingv1.NetworkPolicy{
			ObjectMeta: objectMeta("web-from-api", "np"),
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{MatchLabels: web},
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
				Ingress: []networkingv1.NetworkPolicyIngressRule{{From: []networkingv1.NetworkPolicyPeer{
					{PodSelector: &metav1.LabelSelector{MatchLabels: api}},
					{IPBlock: &networkingv1.IPBlock{CIDR: "10.0.0.0/8"}},
				}}},
			},
		},
		&corev1.ServiceAccount{ObjectMeta: objectMeta("web-sa", "sa")},
		&rbacv1.Role{ObjectMeta: objectMeta("reader", "role"), Rules: []rbacv1.PolicyRule{{Verbs: []string{"get"}}, {Verbs: []string{"list"}}}},
		&rbacv1.RoleBinding{
			ObjectMeta: objectMeta("web-reader", "rb"),
			Subjects:   []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: "web-sa"}},
			RoleRef:    rbacv1.RoleRef{Kind: "Role", Name: "reader"},
		},
		&corev1.ResourceQuota{
			ObjectMeta: objectMeta("pods", "rq"),
			Status: corev1.ResourceQuotaStatus{
				Hard: corev1.ResourceList{corev1.ResourcePods: resource.MustParse("10")},
				Used: corev1.ResourceList{corev1.ResourcePods: resource.MustParse("10")},
			},
		},
	)

	g, err := NewKubeTopologyService(pool).Namespace(context.Background(), "test", testNS)
	if err != nil {
		t.Fatal(err)
	}

	webID := kubeNodeID("Deployment", testNS, "web")
	apiID := kubeNodeID("Deployment", testNS, "api")
	sa := kubeNodeID("ServiceAccount", testNS, "web-sa")
	rb := kubeNodeID("RoleBinding", testNS, "web-reader")
	expected := []struct{ source, target, kind string }{
		{kubeNodeID("HorizontalPodAutoscaler", testNS, "web"), webID, "scales"},
		{kubeNodeID("PodDisruptionBudget", testNS, "web"), webID, "protects"},
		{kubeNodeID("NetworkPolicy", testNS, "web-from-api"), webID, "applies"},
		{apiID, webID, "allows"},
		{webID, sa, "runs-as"},
		{rb, sa, "binds"},
		{rb, kubeNodeID("Role", testNS, "reader"), "grants"},
	}
	for _, e := range expected {
		if !hasEdge(g, e.source, e.target, e.kind) {
			t.Errorf("missing %s edge %s -> %s", e.kind, e.source, e.target)
		}
	}

	nodes := nodeIDs(g)
	if _, ok := nodes[kubeNodeID("ServiceAccount", testNS, "default")]; ok {
		t.Error("unbound default service account should not be drawn")
	}
	if q := nodes[kubeNodeID("ResourceQuota", testNS, "pods")]; q.Status != statusError || q.Summary != "pods 10/10" {
		t.Errorf("full quota should be an error: %+v", q)
	}
	if r := nodes[kubeNodeID("Role", testNS, "reader")]; r.Summary != "2 rules" || r.APIGroup != "rbac.authorization.k8s.io" {
		t.Errorf("unexpected role node: %+v", r)
	}
	if h := nodes[kubeNodeID("HorizontalPodAutoscaler", testNS, "web")]; h.Details["Targets"] != "cpu 80%" {
		t.Errorf("unexpected hpa node: %+v", h)
	}
}

func TestNamespaceGraphFollowsStatefulSetClaimsToStorage(t *testing.T) {
	class := "standard"
	pool := newTestPool(
		&appsv1.StatefulSet{
			ObjectMeta: objectMeta("redis", "sts"),
			Spec: appsv1.StatefulSetSpec{
				Replicas:             replicas(1),
				Template:             corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "redis"}}}},
				VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "data"}}},
			},
		},
		&corev1.PersistentVolumeClaim{
			ObjectMeta: objectMeta("data-redis-0", "pvc"),
			Spec:       corev1.PersistentVolumeClaimSpec{VolumeName: "pv-1", StorageClassName: &class},
			Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound},
		},
		&corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{Name: "pv-1"},
			Spec:       corev1.PersistentVolumeSpec{StorageClassName: class},
			Status:     corev1.PersistentVolumeStatus{Phase: corev1.VolumeBound},
		},
	)

	g, err := NewKubeTopologyService(pool).Namespace(context.Background(), "test", testNS)
	if err != nil {
		t.Fatal(err)
	}

	claim := kubeNodeID("PersistentVolumeClaim", testNS, "data-redis-0")
	pv := kubeNodeID("PersistentVolume", "", "pv-1")
	if !hasEdge(g, kubeNodeID("StatefulSet", testNS, "redis"), claim, "uses") ||
		!hasEdge(g, claim, pv, "bound") ||
		!hasEdge(g, pv, kubeNodeID("StorageClass", "", class), "uses") {
		t.Errorf("missing storage chain: %+v", g.Edges)
	}
}

func TestNamespaceGraphShowsCustomOwnersAndSelectorlessServices(t *testing.T) {
	controller := true
	ro := rollout("web", "ro-web")
	pool := newTestPool(
		ro,
		&appsv1.ReplicaSet{ObjectMeta: objectMeta("web-abc", "rs-web", metav1.OwnerReference{
			APIVersion: "argoproj.io/v1alpha1", Kind: "Rollout", Name: "web", UID: "ro-web", Controller: &controller,
		})},
		runningPod(objectMeta("web-abc-1", "pod-1", ownerRef("ReplicaSet", "web-abc", "rs-web")), "node-a", nil),
		&corev1.Service{ObjectMeta: objectMeta("legacy", "svc")},
		&discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{Name: "legacy-x", Namespace: testNS, Labels: map[string]string{discoveryv1.LabelServiceName: "legacy"}},
			Endpoints:  []discoveryv1.Endpoint{{TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "web-abc-1"}}},
		},
	)

	g, err := NewKubeTopologyService(pool).Namespace(context.Background(), "test", testNS)
	if err != nil {
		t.Fatal(err)
	}

	rolloutID := kubeNodeID("Rollout", testNS, "web")
	node, ok := nodeIDs(g)[rolloutID]
	if !ok || !node.Drillable || node.APIGroup != "argoproj.io" || node.Summary != "1 pods" {
		t.Fatalf("custom owner not drawn as drillable workload: %+v", node)
	}
	if !hasEdge(g, kubeNodeID("Service", testNS, "legacy"), rolloutID, "selects") {
		t.Errorf("selector-less service should link through its endpoints: %+v", g.Edges)
	}

	wg, err := NewKubeTopologyService(pool).Object(context.Background(), "test", testNS, "argoproj.io", "Rollout", "web")
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(wg, rolloutID, kubeNodeID("Pod", testNS, "web-abc-1"), "owns") {
		t.Errorf("custom owner should open straight to its pods: %+v", wg.Edges)
	}
	if root := nodeIDs(wg)[rolloutID]; root.Status != statusHealthy {
		t.Errorf("rollout status should come from its Available condition: %+v", root)
	}
}

func TestBrowserListsKindsAndCustomResources(t *testing.T) {
	pool, clientset := newTestCluster(rollout("web", "ro-web"))
	clientset.Resources = []*metav1.APIResourceList{
		{GroupVersion: "v1", APIResources: []metav1.APIResource{
			{Name: "pods", Kind: "Pod", Namespaced: true, Verbs: []string{"get", "list"}},
			{Name: "pods/log", Kind: "Pod", Namespaced: true, Verbs: []string{"get"}},
			{Name: "bindings", Kind: "Binding", Namespaced: true, Verbs: []string{"create"}},
		}},
		{GroupVersion: "argoproj.io/v1alpha1", APIResources: []metav1.APIResource{
			{Name: "rollouts", Kind: "Rollout", Namespaced: true, Verbs: []string{"get", "list"}},
		}},
	}
	browser := NewKubeBrowserService(pool)

	kinds, err := browser.Kinds(context.Background(), "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(kinds) != 2 || kinds[0].Kind != "Pod" || kinds[1].Kind != "Rollout" || kinds[1].Group != "argoproj.io" {
		t.Errorf("expected listable pods and rollouts only, got %+v", kinds)
	}

	list, err := browser.List(context.Background(), "test", "argoproj.io", "v1alpha1", "rollouts", testNS, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 || list.Items[0].Name != "web" || list.Items[0].Status != statusHealthy || list.Items[0].Group != "argoproj.io" {
		t.Errorf("unexpected objects: %+v", list)
	}
}

func TestBrowserCountsKindsPerNamespaceAndSkipsUnknownKinds(t *testing.T) {
	other := runningPod(objectMeta("elsewhere", "p3"), "node-a", nil)
	other.Namespace = "other"
	deployment := webDeployment()
	deployment.TypeMeta = metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"}
	pool := newTestPool(
		runningPod(objectMeta("a", "p1"), "node-a", nil),
		runningPod(objectMeta("b", "p2"), "node-a", nil),
		other,
		deployment,
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-a"}},
	)
	browser := NewKubeBrowserService(pool)

	counts, err := browser.Counts(context.Background(), "test", testNS, []string{"/Pod", "apps/Deployment", "/Node", "example.com/Missing"})
	if err != nil {
		t.Fatal(err)
	}
	if counts["/Pod"] != 2 || counts["apps/Deployment"] != 1 || counts["/Node"] != 1 {
		t.Errorf("unexpected counts: %+v", counts)
	}
	if _, ok := counts["example.com/Missing"]; ok {
		t.Error("unknown kinds should be left out")
	}

	all, err := browser.Counts(context.Background(), "test", "", []string{"/Pod"})
	if err != nil || all["/Pod"] != 3 {
		t.Errorf("all-namespace pod count = %+v, %v", all, err)
	}

	list, err := browser.List(context.Background(), "test", "apps", "v1", "deployments", testNS, "")
	if err != nil || len(list.Items) != 1 || !list.Items[0].Drillable {
		t.Errorf("deployments should be drillable: %+v, %v", list, err)
	}
}
