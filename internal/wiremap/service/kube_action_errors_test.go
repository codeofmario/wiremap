package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestActionsRejectInvalidRequests(t *testing.T) {
	svc := NewKubeActionService(newTestPool(webDeployment()))
	ctx := context.Background()

	cases := []struct {
		name string
		call func() error
		want string
	}{
		{"negative replicas", func() error { return svc.Scale(ctx, ref("apps", "Deployment", "web"), -1) }, "negative"},
		{"restart a ConfigMap", func() error { return svc.Restart(ctx, ref("", "ConfigMap", "c")) }, "can't be restarted"},
		{"suspend a Deployment", func() error { return svc.SetSuspended(ctx, ref("apps", "Deployment", "web"), true) }, "can't be suspended"},
		{"broken yaml", func() error { return svc.Update(ctx, ref("apps", "Deployment", "web"), "kind: [") }, "invalid yaml"},
		{"renamed object", func() error {
			return svc.Update(ctx, ref("apps", "Deployment", "web"), "kind: Deployment\nmetadata:\n  name: other\n  namespace: shop\n")
		}, "can't be changed"},
		{"unknown kind", func() error { return svc.Delete(ctx, ref("example.com", "Widget", "w")) }, "not served"},
		{"unknown cluster", func() error { return svc.Delete(ctx, ObjectRef{Cluster: "missing", Kind: "Pod"}) }, "not configured"},
		{"missing cronjob", func() error { _, err := svc.TriggerCronJob(ctx, ref("batch", "CronJob", "gone")); return err }, "get cronjob"},
		{"trigger on unknown cluster", func() error {
			_, err := svc.TriggerCronJob(ctx, ObjectRef{Cluster: "missing", Kind: "CronJob"})
			return err
		}, "not configured"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := c.call(); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("expected an error containing %q, got %v", c.want, err)
			}
		})
	}
}

func TestActionsWrapAPIErrors(t *testing.T) {
	pool, _ := newTestCluster(webDeployment(), &corev1.Secret{ObjectMeta: objectMeta("creds", "s")})
	c, _ := pool.Get("test")
	dynamicClient := c.Dynamic.(*dynamicfake.FakeDynamicClient)
	for _, verb := range []string{"delete", "patch", "update", "get"} {
		dynamicClient.PrependReactor(verb, "*", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, k8serrors.NewInternalError(errors.New("boom"))
		})
	}
	svc := NewKubeActionService(pool)
	ctx := context.Background()
	manifest := "kind: Deployment\nmetadata:\n  name: web\n  namespace: shop\n"
	secret := "kind: Secret\nmetadata:\n  name: creds\n  namespace: shop\nstringData:\n  password: " + redactedValue + "\n"

	cases := map[string]func() error{
		"delete":        func() error { return svc.Delete(ctx, ref("apps", "Deployment", "web")) },
		"scale":         func() error { return svc.Scale(ctx, ref("apps", "Deployment", "web"), 2) },
		"restart":       func() error { return svc.Restart(ctx, ref("apps", "Deployment", "web")) },
		"suspend":       func() error { return svc.SetSuspended(ctx, ref("batch", "CronJob", "report"), true) },
		"update":        func() error { return svc.Update(ctx, ref("apps", "Deployment", "web"), manifest) },
		"update secret": func() error { return svc.Update(ctx, ref("", "Secret", "creds"), secret) },
	}
	for name, call := range cases {
		if err := call(); err == nil || !strings.Contains(err.Error(), "boom") {
			t.Errorf("%s: expected the API error, got %v", name, err)
		}
	}
}

func TestTruncateNameKeepsLabelSafeLength(t *testing.T) {
	long := strings.Repeat("a", 70)
	if got := truncateName(long); len(got) != 63 {
		t.Errorf("truncateName kept %d characters", len(got))
	}
	if truncateName("short") != "short" {
		t.Error("short names should be unchanged")
	}
}

func TestCronJobActionsExposeTriggerAndSuspend(t *testing.T) {
	cronJob := &unstructured.Unstructured{Object: map[string]interface{}{"spec": map[string]interface{}{"suspend": true}}}
	a := actionsFor(cronJob, "batch", "CronJob")
	if !a.Trigger || a.Suspended == nil || !*a.Suspended || a.Restart {
		t.Errorf("unexpected cronjob actions: %+v", a)
	}
}

func TestNodeActionsReportFailures(t *testing.T) {
	ctx := context.Background()
	if err := NewKubeNodeActionService(newTestPool()).SetUnschedulable(ctx, "missing", "n", true); err == nil {
		t.Error("unknown cluster should fail")
	}
	if err := NewKubeNodeActionService(newTestPool()).SetUnschedulable(ctx, "test", "missing", true); err == nil || !strings.Contains(err.Error(), "update node") {
		t.Errorf("missing node should fail, got %v", err)
	}

	pool, clientset := newTestCluster(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-a"}})
	clientset.PrependReactor("list", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, k8serrors.NewInternalError(errors.New("boom"))
	})
	if _, err := NewKubeNodeActionService(pool).Drain(ctx, "test", "node-a"); err == nil || !strings.Contains(err.Error(), "list pods") {
		t.Errorf("drain should report list failures, got %v", err)
	}
}

func TestDrainReportsRefusedEvictions(t *testing.T) {
	app := runningPod(objectMeta("app", "p1"), "node-a", nil)
	pool, clientset := newTestCluster(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-a"}}, app)
	clientset.PrependReactor("create", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() != "eviction" {
			return false, nil, nil
		}
		return true, nil, k8serrors.NewTooManyRequests("disruption budget", 0)
	})

	refused, err := NewKubeNodeActionService(pool).Drain(context.Background(), "test", "node-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(refused) != 1 || !strings.HasPrefix(refused[0], testNS+"/app: ") {
		t.Errorf("expected the refused eviction, got %+v", refused)
	}
}

func TestEvictableSkipsMirrorAndFinishedPods(t *testing.T) {
	mirror := runningPod(objectMeta("static", "p1"), "node-a", nil)
	mirror.Annotations = map[string]string{corev1.MirrorPodAnnotationKey: "x"}
	done := runningPod(objectMeta("done", "p2"), "node-a", nil)
	done.Status.Phase = corev1.PodSucceeded

	if evictable(mirror) || evictable(done) {
		t.Error("mirror and finished pods should be left alone")
	}
}

func TestResourceEventsAreOptional(t *testing.T) {
	pool, clientset := newTestCluster(webDeployment())
	clientset.PrependReactor("list", "events", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, k8serrors.NewForbidden(schema.GroupResource{Resource: "events"}, "", nil)
	})

	res, err := NewKubeResourceService(pool).Get(context.Background(), "test", "apps", "Deployment", testNS, "web")
	if err != nil {
		t.Fatal(err)
	}
	if res.Events == nil || len(res.Events) != 0 {
		t.Errorf("forbidden events should be empty, got %+v", res.Events)
	}

	clientset.PrependReactor("list", "events", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, k8serrors.NewInternalError(errors.New("boom"))
	})
	if _, err := NewKubeResourceService(pool).Get(context.Background(), "test", "apps", "Deployment", testNS, "web"); err == nil {
		t.Error("server errors listing events should fail")
	}
	if _, err := NewKubeResourceService(pool).Get(context.Background(), "missing", "apps", "Deployment", testNS, "web"); err == nil {
		t.Error("unknown cluster should fail")
	}
}
