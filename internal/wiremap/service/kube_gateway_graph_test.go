package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/codeofmario/wiremap/internal/wiremap/kube"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/meta/testrestmapper"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	k8stesting "k8s.io/client-go/testing"
)

var gatewayVersion = schema.GroupVersion{Group: gatewayGroup, Version: "v1"}

// newGatewayTestCluster serves the Gateway API's Gateway and HTTPRoute kinds
// next to the built-in ones.
func newGatewayTestCluster(t *testing.T, objects ...runtime.Object) (*kube.ClusterPool, *dynamicfake.FakeDynamicClient) {
	t.Helper()
	// Explicit plurals: the guessed one for Gateway would be "gatewaies"
	resources := map[string]string{"Gateway": "gateways", "HTTPRoute": "httproutes"}

	gateway := meta.NewDefaultRESTMapper([]schema.GroupVersion{gatewayVersion})
	listKinds := map[schema.GroupVersionResource]string{}
	for kind, resource := range resources {
		gateway.AddSpecific(gatewayVersion.WithKind(kind), gatewayVersion.WithResource(resource),
			gatewayVersion.WithResource(strings.ToLower(kind)), meta.RESTScopeNamespace)
		listKinds[gatewayVersion.WithResource(resource)] = kind + "List"
	}

	var typed []runtime.Object
	var custom []*unstructured.Unstructured
	for _, o := range objects {
		if u, ok := o.(*unstructured.Unstructured); ok {
			custom = append(custom, u)
		} else {
			typed = append(typed, o)
		}
	}

	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme.Scheme, listKinds, typed...)
	for _, u := range custom {
		gvr := gatewayVersion.WithResource(resources[u.GetKind()])
		if err := dynamicClient.Tracker().Create(gvr, u, u.GetNamespace()); err != nil {
			t.Fatal(err)
		}
	}

	pool := kube.NewStaticClusterPool(&kube.Cluster{
		Name:      "test",
		Clientset: fake.NewSimpleClientset(typed...),
		Dynamic:   dynamicClient,
		Mapper:    meta.MultiRESTMapper{testrestmapper.TestOnlyStaticRESTMapper(scheme.Scheme), gateway},
	})
	return pool, dynamicClient
}

func gatewayObject(kind, name string, content map[string]interface{}) *unstructured.Unstructured {
	u := &unstructured.Unstructured{Object: content}
	u.SetGroupVersionKind(gatewayVersion.WithKind(kind))
	u.SetNamespace(testNS)
	u.SetName(name)
	return u
}

func acceptedParents(status string) map[string]interface{} {
	return map[string]interface{}{"parents": []interface{}{
		map[string]interface{}{"conditions": []interface{}{map[string]interface{}{"type": "Accepted", "status": status}}},
	}}
}

func TestNamespaceGraphDrawsGatewayRoutes(t *testing.T) {
	pool, _ := newGatewayTestCluster(t,
		&corev1.Service{ObjectMeta: objectMeta("web", "svc")},
		gatewayObject("Gateway", "public", map[string]interface{}{
			"spec": map[string]interface{}{
				"gatewayClassName": "nginx",
				"listeners":        []interface{}{map[string]interface{}{"name": "http"}, map[string]interface{}{"name": "https"}},
			},
			"status": map[string]interface{}{"conditions": []interface{}{map[string]interface{}{"type": "Programmed", "status": "True"}}},
		}),
		gatewayObject("HTTPRoute", "shop", map[string]interface{}{
			"spec": map[string]interface{}{
				"hostnames": []interface{}{"shop.example.com", "www.shop.example.com"},
				"parentRefs": []interface{}{
					map[string]interface{}{"name": "public"},
					map[string]interface{}{"name": "shared", "namespace": "infra"},
					map[string]interface{}{"kind": "Service", "name": "mesh"},
				},
				"rules": []interface{}{map[string]interface{}{"backendRefs": []interface{}{
					map[string]interface{}{"name": "web"},
					map[string]interface{}{"kind": "Bucket", "name": "assets"},
				}}},
			},
			"status": acceptedParents("True"),
		}),
	)

	g, err := NewKubeTopologyService(pool).Namespace(context.Background(), "test", testNS)
	if err != nil {
		t.Fatal(err)
	}

	nodes := nodeIDs(g)
	gw := kubeNodeID("Gateway", testNS, "public")
	shared := kubeNodeID("Gateway", "infra", "shared")
	route := kubeNodeID("HTTPRoute", testNS, "shop")
	if n := nodes[gw]; n.Status != statusHealthy || n.Summary != "nginx · 2 listeners" {
		t.Errorf("gateway: %+v", n)
	}
	if n := nodes[route]; n.Status != statusHealthy || n.Summary != "shop.example.com +1" {
		t.Errorf("route: %+v", n)
	}
	if n := nodes[shared]; n.Status != statusIdle || n.Summary != "in namespace infra" {
		t.Errorf("gateway from another namespace should be drawn as a placeholder: %+v", n)
	}
	if !hasEdge(g, gw, route, "routes") || !hasEdge(g, shared, route, "routes") || !hasEdge(g, route, kubeNodeID("Service", testNS, "web"), "routes") {
		t.Errorf("missing gateway → route → service edges: %+v", g.Edges)
	}
	if _, ok := nodes[kubeNodeID("Gateway", testNS, "mesh")]; ok {
		t.Error("non-Gateway parents should be ignored")
	}
}

func TestGatewayRoutesAreOptional(t *testing.T) {
	pool, dynamicClient := newGatewayTestCluster(t, gatewayObject("Gateway", "public", map[string]interface{}{}))
	dynamicClient.PrependReactor("list", "gateways", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, k8serrors.NewForbidden(schema.GroupResource{Group: gatewayGroup, Resource: "gateways"}, "", nil)
	})

	g, err := NewKubeTopologyService(pool).Namespace(context.Background(), "test", testNS)
	if err != nil {
		t.Fatalf("a forbidden Gateway API should be skipped, got %v", err)
	}
	if len(g.Nodes) != 0 {
		t.Errorf("expected no nodes, got %+v", g.Nodes)
	}

	dynamicClient.PrependReactor("list", "httproutes", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, k8serrors.NewInternalError(errors.New("boom"))
	})
	dynamicClient.PrependReactor("list", "gateways", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, &unstructured.UnstructuredList{}, nil
	})
	if _, err := NewKubeTopologyService(pool).Namespace(context.Background(), "test", testNS); err == nil {
		t.Error("a failing route list should fail the view")
	}
}

func TestRouteStatus(t *testing.T) {
	cases := []struct {
		name        string
		status      map[string]interface{}
		wantStatus  string
		wantSummary string
	}{
		{"not attached", map[string]interface{}{}, statusWarning, "not attached"},
		{"rejected", acceptedParents("False"), statusError, "not accepted"},
		{"accepted", acceptedParents("True"), statusHealthy, "accepted"},
	}
	for _, c := range cases {
		route := gatewayObject("HTTPRoute", "r", map[string]interface{}{"status": c.status})
		status, summary := routeStatus(route)
		if status != c.wantStatus || summary != c.wantSummary {
			t.Errorf("%s: got (%q, %q), want (%q, %q)", c.name, status, summary, c.wantStatus, c.wantSummary)
		}
	}
}

func TestOptionalStepHidesForbiddenAndMissingAPIs(t *testing.T) {
	gr := schema.GroupResource{Resource: "things"}
	if optionalStep(k8serrors.NewForbidden(gr, "", nil), "list things") != nil {
		t.Error("forbidden should be skipped")
	}
	if optionalStep(k8serrors.NewNotFound(gr, ""), "list things") != nil {
		t.Error("missing API should be skipped")
	}
	if err := optionalStep(k8serrors.NewInternalError(errors.New("boom")), "list things"); err == nil {
		t.Error("server errors should fail the view")
	}
}

func TestNamespaceGraphFailsOnRequiredListsAndSkipsOptionalOnes(t *testing.T) {
	forbidden := func(resource string) k8stesting.ReactionFunc {
		return func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, k8serrors.NewForbidden(schema.GroupResource{Resource: resource}, "", nil)
		}
	}
	broken := func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, k8serrors.NewInternalError(errors.New("boom"))
	}

	for _, resource := range []string{"horizontalpodautoscalers", "poddisruptionbudgets", "networkpolicies", "resourcequotas"} {
		pool, clientset := newTestCluster(webDeployment())
		clientset.PrependReactor("list", resource, forbidden(resource))
		if _, err := NewKubeTopologyService(pool).Namespace(context.Background(), "test", testNS); err != nil {
			t.Errorf("forbidden %s should be skipped, got %v", resource, err)
		}
	}

	for _, resource := range []string{"services", "ingresses", "persistentvolumeclaims", "pods", "deployments", "statefulsets", "daemonsets", "cronjobs", "jobs", "replicasets"} {
		pool, clientset := newTestCluster(webDeployment())
		clientset.PrependReactor("list", resource, broken)
		if _, err := NewKubeTopologyService(pool).Namespace(context.Background(), "test", testNS); err == nil {
			t.Errorf("failing to list %s should fail the namespace view", resource)
		}
	}
}

func TestClusterGraphFailsWhenWorkloadsCannotBeCounted(t *testing.T) {
	broken := func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, k8serrors.NewInternalError(errors.New("boom"))
	}
	for _, resource := range []string{"pods", "namespaces", "deployments", "statefulsets", "daemonsets", "cronjobs"} {
		pool, clientset := newTestCluster()
		clientset.PrependReactor("list", resource, broken)
		if _, err := NewKubeTopologyService(pool).Cluster(context.Background(), "test", LensNamespaces); err == nil {
			t.Errorf("failing to list %s should fail the cluster view", resource)
		}
	}
	for _, lens := range []string{LensNodes, LensStorage, LensAccess} {
		pool, clientset := newTestCluster()
		clientset.PrependReactor("list", "*", broken)
		if _, err := NewKubeTopologyService(pool).Cluster(context.Background(), "test", lens); err == nil {
			t.Errorf("%s lens should report list failures", lens)
		}
	}
}
