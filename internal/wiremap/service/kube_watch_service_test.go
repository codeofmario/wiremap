package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	k8stesting "k8s.io/client-go/testing"
)

func waitFor(t *testing.T, timeout time.Duration, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for !done() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
}

func TestDebounceCollapsesBursts(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var calls atomic.Int32
	trigger := debounce(ctx, 50*time.Millisecond, func() { calls.Add(1) })
	for range 5 {
		trigger()
	}
	waitFor(t, time.Second, func() bool { return calls.Load() > 0 })
	time.Sleep(100 * time.Millisecond)
	if calls.Load() != 1 {
		t.Fatalf("a burst should notify once, got %d", calls.Load())
	}

	trigger()
	waitFor(t, time.Second, func() bool { return calls.Load() > 1 })
	if calls.Load() != 2 {
		t.Errorf("a later trigger should notify again, got %d", calls.Load())
	}
}

func TestDebounceStopsWithItsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var calls atomic.Int32
	trigger := debounce(ctx, 50*time.Millisecond, func() { calls.Add(1) })

	trigger()
	cancel()
	time.Sleep(100 * time.Millisecond)
	if calls.Load() != 0 {
		t.Error("a cancelled debounce should not fire")
	}
}

func TestWatchResourceGivesUpOnForbiddenKinds(t *testing.T) {
	client := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	client.PrependReactor("list", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, k8serrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "", nil)
	})

	done := make(chan struct{})
	go func() {
		watchResource(context.Background(), client.Resource(podsGVR).Namespace(testNS), func() {})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("watchResource should return when the kind is forbidden")
	}
}

func TestWatchResourceNotifiesOnChanges(t *testing.T) {
	client := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var calls atomic.Int32
	go watchResource(ctx, client.Resource(podsGVR).Namespace(testNS), func() { calls.Add(1) })
	time.Sleep(100 * time.Millisecond) // let the watch start

	pod := &unstructured.Unstructured{}
	pod.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Pod"))
	pod.SetName("web")
	pod.SetNamespace(testNS)
	if _, err := client.Resource(podsGVR).Namespace(testNS).Create(ctx, pod, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, time.Second, func() bool { return calls.Load() > 0 })
	if calls.Load() == 0 {
		t.Error("expected a notification for the new pod")
	}

}

func TestWatchCoversClusterLensesAndRejectsUnknownClusters(t *testing.T) {
	pool := newTestPool(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNS}})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var changes atomic.Int32
	if err := NewKubeWatchService(pool).Watch(ctx, "test", "", func() { changes.Add(1) }); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond) // let the watches start

	c, _ := pool.Get("test")
	ns := &unstructured.Unstructured{}
	ns.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Namespace"))
	ns.SetName("new-team")
	if _, err := c.Dynamic.Resource(schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}).Create(ctx, ns, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 3*time.Second, func() bool { return changes.Load() > 0 })
	if changes.Load() == 0 {
		t.Error("a new namespace should notify the cluster-level watch")
	}

	if err := NewKubeWatchService(pool).Watch(ctx, "missing", "", func() {}); err == nil {
		t.Error("unknown cluster should fail")
	}
}
