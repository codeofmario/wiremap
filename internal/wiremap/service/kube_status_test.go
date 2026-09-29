package service

import (
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func boolPtr(b bool) *bool { return &b }

func TestWorstStatusPicksTheMoreSevere(t *testing.T) {
	cases := []struct{ a, b, want string }{
		{statusIdle, statusHealthy, statusHealthy},
		{statusError, statusWarning, statusError},
		{statusWarning, statusWarning, statusWarning},
		{"", statusIdle, ""},
	}
	for _, c := range cases {
		if got := worstStatus(c.a, c.b); got != c.want {
			t.Errorf("worstStatus(%q, %q) = %q, want %q", c.a, c.b, got, c.want)
		}
	}
}

func TestPodStatus(t *testing.T) {
	now := metav1.Now()
	cases := []struct {
		name        string
		pod         corev1.Pod
		wantStatus  string
		wantSummary string
	}{
		{
			name: "running and ready",
			pod: corev1.Pod{
				Spec:   corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
				Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{Ready: true}}},
			},
			wantStatus: statusHealthy, wantSummary: "Running · 1/1",
		},
		{
			name: "running but not ready, with restarts",
			pod: corev1.Pod{
				Spec:   corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
				Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{RestartCount: 2}}},
			},
			wantStatus: statusWarning, wantSummary: "NotReady · 0/1 · 2 restarts",
		},
		{
			name: "crash looping",
			pod: corev1.Pod{
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
				Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{
					State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
				}}},
			},
			wantStatus: statusError, wantSummary: "CrashLoopBackOff · 0/1",
		},
		{
			name: "starting containers is not an error",
			pod: corev1.Pod{
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
				Status: corev1.PodStatus{Phase: corev1.PodPending, ContainerStatuses: []corev1.ContainerStatus{{
					State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"}},
				}}},
			},
			wantStatus: statusWarning, wantSummary: "Pending · 0/1",
		},
		{
			name:       "succeeded",
			pod:        corev1.Pod{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "job"}}}, Status: corev1.PodStatus{Phase: corev1.PodSucceeded}},
			wantStatus: statusIdle, wantSummary: "Completed · 0/1",
		},
		{
			name:       "failed",
			pod:        corev1.Pod{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "job"}}}, Status: corev1.PodStatus{Phase: corev1.PodFailed}},
			wantStatus: statusError, wantSummary: "Failed · 0/1",
		},
		{
			name: "terminating",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &now},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
				Status:     corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{Ready: true}}},
			},
			wantStatus: statusWarning, wantSummary: "Terminating · 1/1",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, summary := podStatus(&c.pod)
			if status != c.wantStatus || summary != c.wantSummary {
				t.Errorf("got (%q, %q), want (%q, %q)", status, summary, c.wantStatus, c.wantSummary)
			}
		})
	}
}

func TestReplicaBasedStatuses(t *testing.T) {
	check := func(t *testing.T, name string, gotStatus, gotSummary, wantStatus, wantSummary string) {
		t.Helper()
		if gotStatus != wantStatus || gotSummary != wantSummary {
			t.Errorf("%s: got (%q, %q), want (%q, %q)", name, gotStatus, gotSummary, wantStatus, wantSummary)
		}
	}

	s, sm := replicaStatus(0, 0)
	check(t, "scaled to zero", s, sm, statusIdle, "scaled to 0")
	s, sm = replicaStatus(2, 2)
	check(t, "all ready", s, sm, statusHealthy, "2/2 ready")
	s, sm = replicaStatus(0, 3)
	check(t, "none ready", s, sm, statusError, "0/3 ready")
	s, sm = replicaStatus(1, 3)
	check(t, "some ready", s, sm, statusWarning, "1/3 ready")

	s, sm = deploymentStatus(&appsv1.Deployment{Status: appsv1.DeploymentStatus{ReadyReplicas: 1}})
	check(t, "deployment defaults to one replica", s, sm, statusHealthy, "1/1 ready")
	s, sm = statefulSetStatus(&appsv1.StatefulSet{Spec: appsv1.StatefulSetSpec{Replicas: replicas(2)}, Status: appsv1.StatefulSetStatus{ReadyReplicas: 1}})
	check(t, "statefulset", s, sm, statusWarning, "1/2 ready")
	s, sm = daemonSetStatus(&appsv1.DaemonSet{Status: appsv1.DaemonSetStatus{DesiredNumberScheduled: 3, NumberReady: 3}})
	check(t, "daemonset", s, sm, statusHealthy, "3/3 ready")

	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"deployment.kubernetes.io/revision": "4"}},
		Spec:       appsv1.ReplicaSetSpec{Replicas: replicas(2)},
		Status:     appsv1.ReplicaSetStatus{ReadyReplicas: 2},
	}
	s, sm = replicaSetStatus(rs)
	check(t, "replicaset shows its revision", s, sm, statusHealthy, "rev 4 · 2/2 ready")
	s, sm = replicaSetStatus(&appsv1.ReplicaSet{Spec: appsv1.ReplicaSetSpec{Replicas: replicas(1)}})
	check(t, "bare replicaset", s, sm, statusError, "0/1 ready")
}

func TestJobStatus(t *testing.T) {
	three := int32(3)
	cases := []struct {
		name        string
		job         batchv1.Job
		wantStatus  string
		wantSummary string
	}{
		{"failed", batchv1.Job{Status: batchv1.JobStatus{Failed: 2}}, statusError, "0/1 succeeded · 2 failed"},
		{"completed", batchv1.Job{Status: batchv1.JobStatus{Succeeded: 1}}, statusIdle, "1/1 succeeded"},
		{"running", batchv1.Job{Spec: batchv1.JobSpec{Completions: &three}, Status: batchv1.JobStatus{Active: 2, Succeeded: 1}}, statusHealthy, "1/3 succeeded · 2 active"},
		{"pending", batchv1.Job{}, statusWarning, "0/1 succeeded"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, summary := jobStatus(&c.job)
			if status != c.wantStatus || summary != c.wantSummary {
				t.Errorf("got (%q, %q), want (%q, %q)", status, summary, c.wantStatus, c.wantSummary)
			}
		})
	}
}

func TestCronJobStatus(t *testing.T) {
	cases := []struct {
		name        string
		cronJob     batchv1.CronJob
		wantStatus  string
		wantSummary string
	}{
		{"suspended", batchv1.CronJob{Spec: batchv1.CronJobSpec{Schedule: "@daily", Suspend: boolPtr(true)}}, statusIdle, "@daily · suspended"},
		{"running", batchv1.CronJob{Spec: batchv1.CronJobSpec{Schedule: "@daily"}, Status: batchv1.CronJobStatus{Active: []corev1.ObjectReference{{Name: "a"}}}}, statusHealthy, "@daily · 1 active"},
		{"waiting", batchv1.CronJob{Spec: batchv1.CronJobSpec{Schedule: "@daily", Suspend: boolPtr(false)}}, statusHealthy, "@daily"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, summary := cronJobStatus(&c.cronJob)
			if status != c.wantStatus || summary != c.wantSummary {
				t.Errorf("got (%q, %q), want (%q, %q)", status, summary, c.wantStatus, c.wantSummary)
			}
		})
	}
}

func TestNodeStatus(t *testing.T) {
	ready := func(s corev1.ConditionStatus) []corev1.NodeCondition {
		return []corev1.NodeCondition{{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionFalse}, {Type: corev1.NodeReady, Status: s}}
	}
	cases := []struct {
		name string
		node corev1.Node
		want string
	}{
		{"ready", corev1.Node{Status: corev1.NodeStatus{Conditions: ready(corev1.ConditionTrue)}}, statusHealthy},
		{"not ready", corev1.Node{Status: corev1.NodeStatus{Conditions: ready(corev1.ConditionFalse)}}, statusError},
		{"cordoned", corev1.Node{Spec: corev1.NodeSpec{Unschedulable: true}, Status: corev1.NodeStatus{Conditions: ready(corev1.ConditionTrue)}}, statusWarning},
		{"no ready condition yet", corev1.Node{}, statusWarning},
	}
	for _, c := range cases {
		if got := nodeStatus(&c.node); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestPVCStatus(t *testing.T) {
	bound := corev1.PersistentVolumeClaim{Status: corev1.PersistentVolumeClaimStatus{
		Phase:    corev1.ClaimBound,
		Capacity: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")},
	}}
	cases := []struct {
		name        string
		pvc         corev1.PersistentVolumeClaim
		wantStatus  string
		wantSummary string
	}{
		{"bound", bound, statusHealthy, "Bound · 1Gi"},
		{"lost", corev1.PersistentVolumeClaim{Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimLost}}, statusError, "Lost"},
		{"pending", corev1.PersistentVolumeClaim{Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimPending}}, statusWarning, "Pending"},
	}
	for _, c := range cases {
		status, summary := pvcStatus(&c.pvc)
		if status != c.wantStatus || summary != c.wantSummary {
			t.Errorf("%s: got (%q, %q), want (%q, %q)", c.name, status, summary, c.wantStatus, c.wantSummary)
		}
	}
}

func TestServiceSummary(t *testing.T) {
	cases := []struct {
		name string
		svc  corev1.Service
		want string
	}{
		{"ports", corev1.Service{Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP, Ports: []corev1.ServicePort{
			{Port: 80, Protocol: corev1.ProtocolTCP}, {Port: 53, Protocol: corev1.ProtocolUDP},
		}}}, "ClusterIP · 80/TCP, 53/UDP"},
		{"external name", corev1.Service{Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeExternalName, ExternalName: "db.example.com"}}, "ExternalName → db.example.com"},
		{"headless without ports", corev1.Service{Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP}}, "ClusterIP"},
	}
	for _, c := range cases {
		if got := serviceSummary(&c.svc); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestContainerImagesAndDerefReplicas(t *testing.T) {
	spec := corev1.PodSpec{Containers: []corev1.Container{{Image: "a:1"}, {Image: "b:2"}}}
	if got := containerImages(spec); got != "a:1, b:2" {
		t.Errorf("containerImages = %q", got)
	}
	if derefReplicas(nil) != 1 || derefReplicas(replicas(0)) != 0 {
		t.Error("nil replicas default to 1, explicit values are kept")
	}
}

func TestPhaseStatus(t *testing.T) {
	cases := []struct{ phase, want string }{
		{"Bound", statusHealthy}, {"Released", statusIdle}, {"Failed", statusError}, {"Weird", statusIdle},
	}
	for _, c := range cases {
		status, summary := phaseStatus(c.phase)
		if status != c.want || summary != c.phase {
			t.Errorf("phaseStatus(%q) = (%q, %q)", c.phase, status, summary)
		}
	}
}

func TestAutoscalerStatus(t *testing.T) {
	condition := func(kind autoscalingv2.HorizontalPodAutoscalerConditionType, status corev1.ConditionStatus) autoscalingv2.HorizontalPodAutoscalerCondition {
		return autoscalingv2.HorizontalPodAutoscalerCondition{Type: kind, Status: status}
	}
	cases := []struct {
		name       string
		conditions []autoscalingv2.HorizontalPodAutoscalerCondition
		want       string
	}{
		{"healthy", []autoscalingv2.HorizontalPodAutoscalerCondition{condition(autoscalingv2.AbleToScale, corev1.ConditionTrue)}, statusHealthy},
		{"cannot scale", []autoscalingv2.HorizontalPodAutoscalerCondition{condition(autoscalingv2.AbleToScale, corev1.ConditionFalse)}, statusError},
		{"metrics missing", []autoscalingv2.HorizontalPodAutoscalerCondition{condition(autoscalingv2.ScalingActive, corev1.ConditionFalse)}, statusWarning},
		{"limited is fine", []autoscalingv2.HorizontalPodAutoscalerCondition{condition(autoscalingv2.ScalingLimited, corev1.ConditionFalse)}, statusHealthy},
	}
	for _, c := range cases {
		hpa := &autoscalingv2.HorizontalPodAutoscaler{Status: autoscalingv2.HorizontalPodAutoscalerStatus{Conditions: c.conditions}}
		if got := autoscalerStatus(hpa); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestEventTimePrefersLastTimestamp(t *testing.T) {
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	eventAt := created.Add(time.Hour)
	last := created.Add(2 * time.Hour)

	cases := []struct {
		name  string
		event corev1.Event
		want  time.Time
	}{
		{"last timestamp", corev1.Event{ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.NewTime(created)}, EventTime: metav1.NewMicroTime(eventAt), LastTimestamp: metav1.NewTime(last)}, last},
		{"event time", corev1.Event{ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.NewTime(created)}, EventTime: metav1.NewMicroTime(eventAt)}, eventAt},
		{"creation", corev1.Event{ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.NewTime(created)}}, created},
	}
	for _, c := range cases {
		if got := eventTime(&c.event); !got.Equal(c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
