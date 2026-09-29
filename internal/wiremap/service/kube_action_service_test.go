package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	deploymentGVR = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	secretGVR     = schema.GroupVersionResource{Version: "v1", Resource: "secrets"}
	configMapGVR  = schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
)

func ref(group, kind, name string) ObjectRef {
	return ObjectRef{Cluster: "test", Group: group, Kind: kind, Namespace: testNS, Name: name}
}

func getDynamic(t *testing.T, svc *kubeActionService, gvr schema.GroupVersionResource, name string) *unstructured.Unstructured {
	t.Helper()
	c, _ := svc.pool.Get("test")
	obj, err := c.Dynamic.Resource(gvr).Namespace(testNS).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return obj
}

func TestScaleRestartAndDelete(t *testing.T) {
	svc := NewKubeActionService(newTestPool(webDeployment())).(*kubeActionService)
	ctx := context.Background()

	if err := svc.Scale(ctx, ref("apps", "Deployment", "web"), 5); err != nil {
		t.Fatal(err)
	}
	if replicas, _, _ := unstructured.NestedInt64(getDynamic(t, svc, deploymentGVR, "web").Object, "spec", "replicas"); replicas != 5 {
		t.Errorf("expected 5 replicas, got %d", replicas)
	}

	if err := svc.Restart(ctx, ref("apps", "Deployment", "web")); err != nil {
		t.Fatal(err)
	}
	restartedAt, _, _ := unstructured.NestedString(getDynamic(t, svc, deploymentGVR, "web").Object,
		"spec", "template", "metadata", "annotations", "kubectl.kubernetes.io/restartedAt")
	if restartedAt == "" {
		t.Error("restart should stamp the pod template")
	}
	if err := svc.Restart(ctx, ref("", "ConfigMap", "x")); err == nil {
		t.Error("configmaps can't be restarted")
	}
	if err := svc.Scale(ctx, ref("apps", "Deployment", "web"), -1); err == nil {
		t.Error("negative replicas should be rejected")
	}

	if err := svc.Delete(ctx, ref("apps", "Deployment", "web")); err != nil {
		t.Fatal(err)
	}
	c, _ := svc.pool.Get("test")
	if _, err := c.Dynamic.Resource(deploymentGVR).Namespace(testNS).Get(ctx, "web", metav1.GetOptions{}); err == nil {
		t.Error("deployment should be deleted")
	}
}

func TestUpdateAppliesEditedManifest(t *testing.T) {
	pool := newTestPool(&corev1.ConfigMap{ObjectMeta: objectMeta("app-config", "cm"), Data: map[string]string{"MODE": "dev"}})
	svc := NewKubeActionService(pool).(*kubeActionService)

	manifest := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: app-config\n  namespace: shop\ndata:\n  MODE: prod\n"
	if err := svc.Update(context.Background(), ref("", "ConfigMap", "app-config"), manifest); err != nil {
		t.Fatal(err)
	}
	if mode, _, _ := unstructured.NestedString(getDynamic(t, svc, configMapGVR, "app-config").Object, "data", "MODE"); mode != "prod" {
		t.Errorf("expected updated value, got %q", mode)
	}

	renamed := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: other\n  namespace: shop\n"
	if err := svc.Update(context.Background(), ref("", "ConfigMap", "app-config"), renamed); err == nil {
		t.Error("renaming through an edit should be rejected")
	}
}

func TestSecretEditKeepsRedactedValues(t *testing.T) {
	pool := newTestPool(&corev1.Secret{
		ObjectMeta: objectMeta("db", "sec"),
		Data:       map[string][]byte{"password": []byte("hunter2"), "user": []byte("admin"), "old": []byte("x")},
	})
	svc := NewKubeActionService(pool).(*kubeActionService)

	// As served by the resource view: values redacted; the user changed "user" and dropped "old"
	manifest := "apiVersion: v1\nkind: Secret\nmetadata:\n  name: db\n  namespace: shop\nstringData:\n  password: <redacted>\n  user: root\n"
	if err := svc.Update(context.Background(), ref("", "Secret", "db"), manifest); err != nil {
		t.Fatal(err)
	}

	secret := getDynamic(t, svc, secretGVR, "db")
	data, _, _ := unstructured.NestedStringMap(secret.Object, "data")
	stringData, _, _ := unstructured.NestedStringMap(secret.Object, "stringData")
	if data["password"] != "aHVudGVyMg==" {
		t.Errorf("redacted value should keep the stored password, got %q", data["password"])
	}
	if _, ok := data["old"]; ok {
		t.Error("key removed in the edit should be deleted")
	}
	if stringData["user"] != "root" {
		t.Errorf("edited value should be written, got %+v", stringData)
	}
}

func TestTriggerAndSuspendCronJob(t *testing.T) {
	pool := newTestPool(&batchv1.CronJob{
		ObjectMeta: objectMeta("report", "cj-report"),
		Spec:       batchv1.CronJobSpec{Schedule: "@daily", JobTemplate: batchv1.JobTemplateSpec{Spec: batchv1.JobSpec{}}},
	})
	svc := NewKubeActionService(pool).(*kubeActionService)
	ctx := context.Background()

	name, err := svc.TriggerCronJob(ctx, ref("batch", "CronJob", "report"))
	if err != nil {
		t.Fatal(err)
	}
	c, _ := pool.Get("test")
	job, err := c.Clientset.BatchV1().Jobs(testNS).Get(ctx, name, metav1.GetOptions{})
	if err != nil || !ownedBy(job.OwnerReferences, "cj-report") {
		t.Fatalf("manual job should be owned by the cronjob: %v %+v", err, job)
	}

	if err := svc.SetSuspended(ctx, ref("batch", "CronJob", "report"), true); err != nil {
		t.Fatal(err)
	}
	cj, _ := c.Dynamic.Resource(schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"}).
		Namespace(testNS).Get(ctx, "report", metav1.GetOptions{})
	if suspended, _, _ := unstructured.NestedBool(cj.Object, "spec", "suspend"); !suspended {
		t.Error("cronjob should be suspended")
	}
}

func TestDrainCordonsAndSkipsDaemonSetPods(t *testing.T) {
	daemon := runningPod(objectMeta("agent", "p1", ownerRef("DaemonSet", "agent", "ds")), "node-a", nil)
	app := runningPod(objectMeta("app", "p2", ownerRef("ReplicaSet", "app", "rs")), "node-a", nil)
	pool := newTestPool(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-a"}}, daemon, app)

	if !evictable(app) || evictable(daemon) {
		t.Error("only non-DaemonSet pods should be evicted")
	}

	if _, err := NewKubeNodeActionService(pool).Drain(context.Background(), "test", "node-a"); err != nil {
		t.Fatal(err)
	}
	c, _ := pool.Get("test")
	node, _ := c.Clientset.CoreV1().Nodes().Get(context.Background(), "node-a", metav1.GetOptions{})
	if !node.Spec.Unschedulable {
		t.Error("drain should cordon the node")
	}
}

func TestActionsForObject(t *testing.T) {
	deployment := &unstructured.Unstructured{Object: map[string]interface{}{"spec": map[string]interface{}{"replicas": int64(3)}}}
	actions := actionsFor(deployment, "apps", "Deployment")
	if actions.Replicas == nil || *actions.Replicas != 3 || !actions.Restart || !actions.Edit || !actions.Delete {
		t.Errorf("unexpected deployment actions: %+v", actions)
	}

	node := &unstructured.Unstructured{Object: map[string]interface{}{"spec": map[string]interface{}{"unschedulable": true}}}
	if a := actionsFor(node, "", "Node"); a.Unschedulable == nil || !*a.Unschedulable || a.Replicas != nil {
		t.Errorf("unexpected node actions: %+v", a)
	}
}

func TestAccessGraphHidesSystemBindings(t *testing.T) {
	pool := newTestPool(
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "system:kube-dns"},
			Subjects:   []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: "kube-dns", Namespace: "kube-system"}},
			RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "system:kube-dns"},
		},
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "devs-view"},
			Subjects:   []rbacv1.Subject{{Kind: rbacv1.GroupKind, Name: "devs"}},
			RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "view"},
		},
		&rbacv1.RoleBinding{
			ObjectMeta: objectMeta("ci-deployer", "rb"),
			Subjects:   []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: "ci"}},
			RoleRef:    rbacv1.RoleRef{Kind: "Role", Name: "deployer"},
		},
	)

	g, err := NewKubeTopologyService(pool).Cluster(context.Background(), "test", LensAccess)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := nodeIDs(g)[kubeNodeID("ClusterRoleBinding", "", "system:kube-dns")]; ok {
		t.Error("system bindings should be hidden")
	}
	crb := kubeNodeID("ClusterRoleBinding", "", "devs-view")
	rb := kubeNodeID("RoleBinding", testNS, "ci-deployer")
	if !hasEdge(g, crb, kubeNodeID("Group", "", "devs"), "binds") || !hasEdge(g, crb, kubeNodeID("ClusterRole", "", "view"), "grants") ||
		!hasEdge(g, rb, kubeNodeID("ServiceAccount", testNS, "ci"), "binds") || !hasEdge(g, rb, kubeNodeID("Role", testNS, "deployer"), "grants") {
		t.Errorf("missing access edges: %+v", g.Edges)
	}
}

func TestWatchNotifiesOnChange(t *testing.T) {
	pool := newTestPool(webDeployment())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var changes atomic.Int32
	if err := NewKubeWatchService(pool).Watch(ctx, "test", testNS, func() { changes.Add(1) }); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond) // let the watches start

	c, _ := pool.Get("test")
	scaled := webDeployment()
	scaled.Spec.Replicas = replicas(4)
	obj, _ := toUnstructured(scaled)
	if _, err := c.Dynamic.Resource(deploymentGVR).Namespace(testNS).Update(ctx, obj, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for changes.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if changes.Load() == 0 {
		t.Fatal("expected a change notification")
	}
}

func toUnstructured(d *appsv1.Deployment) (*unstructured.Unstructured, error) {
	d.APIVersion, d.Kind = "apps/v1", "Deployment"
	obj, err := runtimeToUnstructured(d)
	return &unstructured.Unstructured{Object: obj}, err
}
