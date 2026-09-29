package service

import (
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

// Graph node statuses, ordered from least to most severe.
const (
	statusIdle    = "idle"
	statusHealthy = "healthy"
	statusWarning = "warning"
	statusError   = "error"
)

var statusSeverity = map[string]int{statusIdle: 0, statusHealthy: 1, statusWarning: 2, statusError: 3}

// worstStatus returns the more severe of two statuses.
func worstStatus(a, b string) string {
	if statusSeverity[b] > statusSeverity[a] {
		return b
	}
	return a
}

// Container waiting reasons that are part of a normal pod start.
var benignWaitingReasons = map[string]bool{"ContainerCreating": true, "PodInitializing": true}

func podStatus(pod *corev1.Pod) (string, string) {
	ready, restarts := 0, int32(0)
	reason := string(pod.Status.Phase)
	status := statusWarning

	for _, cs := range pod.Status.ContainerStatuses {
		restarts += cs.RestartCount
		if cs.Ready {
			ready++
		}
		if w := cs.State.Waiting; w != nil && w.Reason != "" && !benignWaitingReasons[w.Reason] {
			reason = w.Reason
			status = statusError
		}
	}

	if status != statusError {
		switch pod.Status.Phase {
		case corev1.PodRunning:
			if ready == len(pod.Spec.Containers) {
				status = statusHealthy
			} else {
				reason = "NotReady"
			}
		case corev1.PodSucceeded:
			status, reason = statusIdle, "Completed"
		case corev1.PodFailed:
			status = statusError
		}
	}

	if pod.DeletionTimestamp != nil {
		status, reason = statusWarning, "Terminating"
	}

	summary := fmt.Sprintf("%s · %d/%d", reason, ready, len(pod.Spec.Containers))
	if restarts > 0 {
		summary += fmt.Sprintf(" · %d restarts", restarts)
	}
	return status, summary
}

// replicaStatus rates a workload by how many of its desired replicas are ready.
func replicaStatus(ready, desired int32) (string, string) {
	summary := fmt.Sprintf("%d/%d ready", ready, desired)
	switch {
	case desired == 0:
		return statusIdle, "scaled to 0"
	case ready == desired:
		return statusHealthy, summary
	case ready == 0:
		return statusError, summary
	default:
		return statusWarning, summary
	}
}

func deploymentStatus(d *appsv1.Deployment) (string, string) {
	return replicaStatus(d.Status.ReadyReplicas, derefReplicas(d.Spec.Replicas))
}

func statefulSetStatus(s *appsv1.StatefulSet) (string, string) {
	return replicaStatus(s.Status.ReadyReplicas, derefReplicas(s.Spec.Replicas))
}

func daemonSetStatus(d *appsv1.DaemonSet) (string, string) {
	return replicaStatus(d.Status.NumberReady, d.Status.DesiredNumberScheduled)
}

func replicaSetStatus(r *appsv1.ReplicaSet) (string, string) {
	status, summary := replicaStatus(r.Status.ReadyReplicas, derefReplicas(r.Spec.Replicas))
	if revision := r.Annotations["deployment.kubernetes.io/revision"]; revision != "" {
		summary = "rev " + revision + " · " + summary
	}
	return status, summary
}

func jobStatus(j *batchv1.Job) (string, string) {
	completions := int32(1)
	if j.Spec.Completions != nil {
		completions = *j.Spec.Completions
	}
	summary := fmt.Sprintf("%d/%d succeeded", j.Status.Succeeded, completions)
	switch {
	case j.Status.Failed > 0 && j.Status.Active == 0 && j.Status.Succeeded < completions:
		return statusError, summary + fmt.Sprintf(" · %d failed", j.Status.Failed)
	case j.Status.Succeeded >= completions:
		return statusIdle, summary
	case j.Status.Active > 0:
		return statusHealthy, summary + fmt.Sprintf(" · %d active", j.Status.Active)
	default:
		return statusWarning, summary
	}
}

func cronJobStatus(c *batchv1.CronJob) (string, string) {
	if c.Spec.Suspend != nil && *c.Spec.Suspend {
		return statusIdle, c.Spec.Schedule + " · suspended"
	}
	if len(c.Status.Active) > 0 {
		return statusHealthy, fmt.Sprintf("%s · %d active", c.Spec.Schedule, len(c.Status.Active))
	}
	return statusHealthy, c.Spec.Schedule
}

func nodeStatus(n *corev1.Node) string {
	if n.Spec.Unschedulable {
		return statusWarning
	}
	for _, cond := range n.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			if cond.Status == corev1.ConditionTrue {
				return statusHealthy
			}
			return statusError
		}
	}
	return statusWarning
}

func pvcStatus(p *corev1.PersistentVolumeClaim) (string, string) {
	summary := string(p.Status.Phase)
	if size, ok := p.Status.Capacity[corev1.ResourceStorage]; ok {
		summary += " · " + size.String()
	}
	switch p.Status.Phase {
	case corev1.ClaimBound:
		return statusHealthy, summary
	case corev1.ClaimLost:
		return statusError, summary
	default:
		return statusWarning, summary
	}
}

func serviceSummary(s *corev1.Service) string {
	ports := make([]string, 0, len(s.Spec.Ports))
	for _, p := range s.Spec.Ports {
		ports = append(ports, fmt.Sprintf("%d/%s", p.Port, p.Protocol))
	}
	summary := string(s.Spec.Type)
	if s.Spec.Type == corev1.ServiceTypeExternalName {
		return summary + " → " + s.Spec.ExternalName
	}
	if len(ports) > 0 {
		summary += " · " + strings.Join(ports, ", ")
	}
	return summary
}

func derefReplicas(replicas *int32) int32 {
	if replicas == nil {
		return 1
	}
	return *replicas
}

func containerImages(spec corev1.PodSpec) string {
	images := make([]string, 0, len(spec.Containers))
	for _, c := range spec.Containers {
		images = append(images, c.Image)
	}
	return strings.Join(images, ", ")
}
