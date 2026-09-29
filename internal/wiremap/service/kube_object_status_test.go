package service

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func typedUnstructured(t *testing.T, obj runtime.Object, gvk schema.GroupVersionKind) *unstructured.Unstructured {
	t.Helper()
	m, err := runtimeToUnstructured(obj)
	if err != nil {
		t.Fatal(err)
	}
	u := &unstructured.Unstructured{Object: m}
	u.SetGroupVersionKind(gvk)
	return u
}

func TestObjectStatusUsesTypedRulesForBuiltInKinds(t *testing.T) {
	cases := []struct {
		name        string
		obj         runtime.Object
		gvk         schema.GroupVersionKind
		wantStatus  string
		wantSummary string
	}{
		{
			name:       "pod",
			obj:        &corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodSucceeded}},
			gvk:        corev1.SchemeGroupVersion.WithKind("Pod"),
			wantStatus: statusIdle, wantSummary: "Completed · 0/0",
		},
		{
			name: "node",
			obj: &corev1.Node{Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
				NodeInfo:   corev1.NodeSystemInfo{KubeletVersion: "v1.34.0"},
			}},
			gvk:        corev1.SchemeGroupVersion.WithKind("Node"),
			wantStatus: statusHealthy, wantSummary: "v1.34.0",
		},
		{
			name:       "pvc",
			obj:        &corev1.PersistentVolumeClaim{Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimPending}},
			gvk:        corev1.SchemeGroupVersion.WithKind("PersistentVolumeClaim"),
			wantStatus: statusWarning, wantSummary: "Pending",
		},
		{
			name:       "service",
			obj:        &corev1.Service{Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP, Ports: []corev1.ServicePort{{Port: 80, Protocol: corev1.ProtocolTCP}}}},
			gvk:        corev1.SchemeGroupVersion.WithKind("Service"),
			wantStatus: statusHealthy, wantSummary: "ClusterIP · 80/TCP",
		},
		{
			name:       "deployment",
			obj:        &appsv1.Deployment{Spec: appsv1.DeploymentSpec{Replicas: replicas(2)}, Status: appsv1.DeploymentStatus{ReadyReplicas: 1}},
			gvk:        appsv1.SchemeGroupVersion.WithKind("Deployment"),
			wantStatus: statusWarning, wantSummary: "1/2 ready",
		},
		{
			name:       "statefulset",
			obj:        &appsv1.StatefulSet{Spec: appsv1.StatefulSetSpec{Replicas: replicas(0)}},
			gvk:        appsv1.SchemeGroupVersion.WithKind("StatefulSet"),
			wantStatus: statusIdle, wantSummary: "scaled to 0",
		},
		{
			name:       "daemonset",
			obj:        &appsv1.DaemonSet{Status: appsv1.DaemonSetStatus{DesiredNumberScheduled: 2}},
			gvk:        appsv1.SchemeGroupVersion.WithKind("DaemonSet"),
			wantStatus: statusError, wantSummary: "0/2 ready",
		},
		{
			name:       "replicaset",
			obj:        &appsv1.ReplicaSet{Spec: appsv1.ReplicaSetSpec{Replicas: replicas(1)}, Status: appsv1.ReplicaSetStatus{ReadyReplicas: 1}},
			gvk:        appsv1.SchemeGroupVersion.WithKind("ReplicaSet"),
			wantStatus: statusHealthy, wantSummary: "1/1 ready",
		},
		{
			name:       "job",
			obj:        &batchv1.Job{Status: batchv1.JobStatus{Succeeded: 1}},
			gvk:        batchv1.SchemeGroupVersion.WithKind("Job"),
			wantStatus: statusIdle, wantSummary: "1/1 succeeded",
		},
		{
			name:       "cronjob",
			obj:        &batchv1.CronJob{Spec: batchv1.CronJobSpec{Schedule: "@hourly"}},
			gvk:        batchv1.SchemeGroupVersion.WithKind("CronJob"),
			wantStatus: statusHealthy, wantSummary: "@hourly",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, summary := objectStatus(typedUnstructured(t, c.obj, c.gvk))
			if status != c.wantStatus || summary != c.wantSummary {
				t.Errorf("got (%q, %q), want (%q, %q)", status, summary, c.wantStatus, c.wantSummary)
			}
		})
	}
}

func TestObjectStatusFallsBackToPhaseAndConditions(t *testing.T) {
	withStatus := func(status map[string]interface{}) *unstructured.Unstructured {
		u := &unstructured.Unstructured{Object: map[string]interface{}{"status": status}}
		u.SetGroupVersionKind(schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Widget"})
		return u
	}
	condition := func(kind, status, reason string) map[string]interface{} {
		return map[string]interface{}{"conditions": []interface{}{
			map[string]interface{}{"type": "Unrelated", "status": "False"},
			map[string]interface{}{"type": kind, "status": status, "reason": reason},
		}}
	}

	cases := []struct {
		name        string
		obj         *unstructured.Unstructured
		wantStatus  string
		wantSummary string
	}{
		{"known phase", withStatus(map[string]interface{}{"phase": "Running"}), statusHealthy, "Running"},
		{"unknown phase", withStatus(map[string]interface{}{"phase": "Hibernating"}), statusIdle, "Hibernating"},
		{"ready condition", withStatus(condition("Ready", "True", "")), statusHealthy, "Ready"},
		{"failed condition", withStatus(condition("Available", "False", "NoReplicas")), statusError, "NotAvailable NoReplicas"},
		{"unknown condition", withStatus(condition("Programmed", "Unknown", "Pending")), statusWarning, "Programmed unknown Pending"},
		{"nothing to go on", withStatus(map[string]interface{}{}), statusIdle, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, summary := objectStatus(c.obj)
			if status != c.wantStatus || summary != c.wantSummary {
				t.Errorf("got (%q, %q), want (%q, %q)", status, summary, c.wantStatus, c.wantSummary)
			}
		})
	}
}

func TestObjectStatusFallsBackWhenTypedConversionFails(t *testing.T) {
	// replicas as a string can't convert to a Deployment; the generic rules still apply
	u := &unstructured.Unstructured{Object: map[string]interface{}{
		"spec":   map[string]interface{}{"replicas": "two"},
		"status": map[string]interface{}{"phase": "Failed"},
	}}
	u.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("Deployment"))

	status, summary := objectStatus(u)
	if status != statusError || summary != "Failed" {
		t.Errorf("got (%q, %q)", status, summary)
	}
}

func TestObjectMetaHelpersIgnoreEmptyValues(t *testing.T) {
	if apiGroup("apps/v1") != "apps" || apiGroup("v1") != "" {
		t.Error("apiGroup should strip the version and treat core as empty")
	}
	if !ownedBy([]metav1.OwnerReference{{UID: "a"}, {UID: "b"}}, "b") || ownedBy(nil, "a") {
		t.Error("ownedBy should match owner UIDs only")
	}
}
