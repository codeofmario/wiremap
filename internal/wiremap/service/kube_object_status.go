package service

import (
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// objectStatus rates any object: built-in kinds reuse their typed rules, everything
// else (including CRDs) falls back to the status.phase / status.conditions conventions.
func objectStatus(u *unstructured.Unstructured) (string, string) {
	switch u.GroupVersionKind().GroupKind().String() {
	case "Pod":
		return typedStatus(u, &corev1.Pod{}, func(p *corev1.Pod) (string, string) { return podStatus(p) })
	case "Node":
		return typedStatus(u, &corev1.Node{}, func(n *corev1.Node) (string, string) { return nodeStatus(n), n.Status.NodeInfo.KubeletVersion })
	case "PersistentVolumeClaim":
		return typedStatus(u, &corev1.PersistentVolumeClaim{}, pvcStatus)
	case "Service":
		return typedStatus(u, &corev1.Service{}, func(svc *corev1.Service) (string, string) { return statusHealthy, serviceSummary(svc) })
	case "Deployment.apps":
		return typedStatus(u, &appsv1.Deployment{}, deploymentStatus)
	case "StatefulSet.apps":
		return typedStatus(u, &appsv1.StatefulSet{}, statefulSetStatus)
	case "DaemonSet.apps":
		return typedStatus(u, &appsv1.DaemonSet{}, daemonSetStatus)
	case "ReplicaSet.apps":
		return typedStatus(u, &appsv1.ReplicaSet{}, replicaSetStatus)
	case "Job.batch":
		return typedStatus(u, &batchv1.Job{}, jobStatus)
	case "CronJob.batch":
		return typedStatus(u, &batchv1.CronJob{}, cronJobStatus)
	}
	return genericStatus(u)
}

func typedStatus[T any](u *unstructured.Unstructured, typed *T, rate func(*T) (string, string)) (string, string) {
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, typed); err != nil {
		return genericStatus(u)
	}
	return rate(typed)
}

var phaseStatuses = map[string]string{
	"running": statusHealthy, "active": statusHealthy, "bound": statusHealthy, "available": statusHealthy,
	"ready": statusHealthy, "succeeded": statusIdle, "completed": statusIdle, "released": statusIdle,
	"pending": statusWarning, "terminating": statusWarning, "progressing": statusWarning,
	"failed": statusError, "lost": statusError, "error": statusError,
}

// Conditions whose True value means the object is working.
var readyConditions = []string{"Ready", "Available", "Accepted", "Programmed", "Established", "Healthy"}

func genericStatus(u *unstructured.Unstructured) (string, string) {
	if phase, _, _ := unstructured.NestedString(u.Object, "status", "phase"); phase != "" {
		if status, ok := phaseStatuses[strings.ToLower(phase)]; ok {
			return status, phase
		}
		return statusIdle, phase
	}

	conditions, _, _ := unstructured.NestedSlice(u.Object, "status", "conditions")
	for _, want := range readyConditions {
		for _, raw := range conditions {
			cond, ok := raw.(map[string]interface{})
			if !ok || cond["type"] != want {
				continue
			}
			reason, _ := cond["reason"].(string)
			switch cond["status"] {
			case "True":
				return statusHealthy, want
			case "False":
				return statusError, strings.TrimSpace("Not" + want + " " + reason)
			default:
				return statusWarning, strings.TrimSpace(want + " unknown " + reason)
			}
		}
	}

	return statusIdle, ""
}
