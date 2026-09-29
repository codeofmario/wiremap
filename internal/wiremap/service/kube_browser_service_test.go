package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/codeofmario/wiremap/internal/wiremap/kube"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	k8stesting "k8s.io/client-go/testing"
)

var podsGVR = schema.GroupVersionResource{Version: "v1", Resource: "pods"}

func podList(names ...string) *unstructured.UnstructuredList {
	list := &unstructured.UnstructuredList{}
	for _, name := range names {
		u := unstructured.Unstructured{}
		u.SetName(name)
		list.Items = append(list.Items, u)
	}
	return list
}

// pagedDynamic answers the first list with one item and a continue token, like
// an API server paginating with limit=1, and full lists afterwards.
func pagedDynamic(remaining *int64, total int) *dynamicfake.FakeDynamicClient {
	client := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	calls := 0
	client.PrependReactor("list", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		calls++
		if calls == 1 {
			page := podList("a")
			page.SetContinue("next")
			page.SetRemainingItemCount(remaining)
			return true, page, nil
		}
		names := make([]string, total)
		for i := range names {
			names[i] = string(rune('a' + i))
		}
		return true, podList(names...), nil
	})
	return client
}

func TestCountObjectsUsesTheRemainingItemCount(t *testing.T) {
	remaining := int64(41)
	c := &kube.Cluster{Dynamic: pagedDynamic(&remaining, 0)}

	count, err := countObjects(context.Background(), c, podsGVR, testNS)
	if err != nil || count != 42 {
		t.Errorf("count = %d, %v; want 42", count, err)
	}
}

func TestCountObjectsFallsBackToAFullList(t *testing.T) {
	c := &kube.Cluster{Dynamic: pagedDynamic(nil, 5)}

	count, err := countObjects(context.Background(), c, podsGVR, testNS)
	if err != nil || count != 5 {
		t.Errorf("count = %d, %v; want 5", count, err)
	}
}

func TestCountObjectsReportsListErrors(t *testing.T) {
	client := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	client.PrependReactor("list", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, k8serrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "", nil)
	})

	if _, err := countObjects(context.Background(), &kube.Cluster{Dynamic: client}, podsGVR, testNS); err == nil {
		t.Error("expected the list error")
	}
}

func TestCountsSkipKindsThatCannotBeListed(t *testing.T) {
	pool, _ := newTestCluster(runningPod(objectMeta("a", "p1"), "node-a", nil), webDeployment())
	c, _ := pool.Get("test")
	c.Dynamic.(*dynamicfake.FakeDynamicClient).PrependReactor("list", "deployments", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, k8serrors.NewForbidden(schema.GroupResource{Group: "apps", Resource: "deployments"}, "", nil)
	})

	counts, err := NewKubeBrowserService(pool).Counts(context.Background(), "test", testNS, []string{"/Pod", "apps/Deployment"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := counts["apps/Deployment"]; ok || counts["/Pod"] != 1 {
		t.Errorf("forbidden kinds should be left out: %+v", counts)
	}
}

func TestCountsRejectMalformedKinds(t *testing.T) {
	_, err := NewKubeBrowserService(newTestPool()).Counts(context.Background(), "test", testNS, []string{"Pod"})
	if err == nil || !strings.Contains(err.Error(), "group/Kind") {
		t.Errorf("expected a bad request, got %v", err)
	}
	if _, err := NewKubeBrowserService(newTestPool()).Counts(context.Background(), "missing", "", nil); err == nil {
		t.Error("unknown cluster should fail")
	}
}

func TestBrowserListValidatesAndWrapsErrors(t *testing.T) {
	browser := NewKubeBrowserService(newTestPool())
	ctx := context.Background()

	if _, err := browser.List(ctx, "test", "", "", "pods", testNS, ""); err == nil || !strings.Contains(err.Error(), "required") {
		t.Errorf("missing version should be a bad request, got %v", err)
	}
	if _, err := browser.List(ctx, "missing", "", "v1", "pods", testNS, ""); err == nil {
		t.Error("unknown cluster should fail")
	}
	if _, err := browser.Kinds(ctx, "missing"); err == nil {
		t.Error("unknown cluster should fail")
	}

	pool, _ := newTestCluster()
	c, _ := pool.Get("test")
	c.Dynamic.(*dynamicfake.FakeDynamicClient).PrependReactor("list", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, k8serrors.NewInternalError(errors.New("boom"))
	})
	if _, err := NewKubeBrowserService(pool).List(ctx, "test", "", "v1", "pods", testNS, ""); err == nil || !strings.Contains(err.Error(), "list pods") {
		t.Errorf("expected a wrapped list error, got %v", err)
	}
}

func TestObjectDtoMarksDrillableKinds(t *testing.T) {
	svc := &unstructured.Unstructured{}
	svc.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "Service"})
	svc.SetName("web")
	svc.SetNamespace(testNS)
	svc.SetCreationTimestamp(metav1.Now())

	role := &unstructured.Unstructured{}
	role.SetGroupVersionKind(schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "Role"})

	if d := objectDto(svc, ""); !d.Drillable || d.Kind != "Service" || d.Namespace != testNS || d.Created == "" {
		t.Errorf("services open their pods: %+v", d)
	}
	if objectDto(role, "rbac.authorization.k8s.io").Drillable {
		t.Error("roles have nothing to open")
	}
}
