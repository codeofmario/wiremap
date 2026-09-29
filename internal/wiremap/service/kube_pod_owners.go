package service

import (
	"context"
	"fmt"

	"github.com/codeofmario/wiremap/internal/wiremap/dto"
	"github.com/codeofmario/wiremap/internal/wiremap/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// podTemplateOwner is anything at namespace level that runs pods: a workload,
// a custom controller's object, or a bare pod.
type podTemplateOwner struct {
	node   dto.KubeGraphNodeDto
	labels map[string]string
	spec   corev1.PodSpec
	// PVC name prefixes created from StatefulSet volumeClaimTemplates
	claimPrefixes []string
}

// namespaceOwners is the result of attributing every pod in a namespace to the
// top-level object that controls it.
type namespaceOwners struct {
	owners []*podTemplateOwner
	// Graph node ID of the owner of each pod, by pod name
	podOwner map[string]string
}

// Owner chains longer than this are treated as broken
const maxOwnerDepth = 5

func (s *kubeTopologyService) listPodOwners(ctx context.Context, c *kube.Cluster, namespace string) (*namespaceOwners, error) {
	result := &namespaceOwners{podOwner: map[string]string{}}
	// Graph node IDs of the workloads drawn at this level, by UID
	shown := map[types.UID]string{}
	// Owner references of intermediate controllers (ReplicaSets, Jobs), by UID
	intermediate := map[types.UID][]metav1.OwnerReference{}

	addWorkload := func(uid types.UID, kind, name, status, summary string, tmpl corev1.PodTemplateSpec) *podTemplateOwner {
		owner := &podTemplateOwner{
			node: dto.KubeGraphNodeDto{
				ID:        kubeNodeID(kind, namespace, name),
				Kind:      kind,
				Name:      name,
				Namespace: namespace,
				Status:    status,
				Summary:   summary,
				Drillable: true,
				Weight:    1,
				Details:   map[string]string{"Images": containerImages(tmpl.Spec)},
			},
			labels: tmpl.Labels,
			spec:   tmpl.Spec,
		}
		result.owners = append(result.owners, owner)
		shown[uid] = owner.node.ID
		return owner
	}

	if err := s.listWorkloads(ctx, c, namespace, addWorkload, intermediate); err != nil {
		return nil, err
	}

	pods, err := c.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, kube.WrapError(err, "list pods")
	}

	// Pods whose top owner isn't a drawn workload (custom controllers, bare ReplicaSets)
	generic := map[string]*podTemplateOwner{}
	for i := range pods.Items {
		pod := &pods.Items[i]
		if len(pod.OwnerReferences) == 0 {
			owner := &podTemplateOwner{node: podNode(pod, ""), labels: pod.Labels, spec: pod.Spec}
			result.owners = append(result.owners, owner)
			result.podOwner[pod.Name] = owner.node.ID
			continue
		}

		top, id := topOwner(pod.OwnerReferences, shown, intermediate)
		if id != "" {
			result.podOwner[pod.Name] = id
			continue
		}

		id = kubeNodeID(top.Kind, namespace, top.Name)
		owner, ok := generic[id]
		if !ok {
			owner = &podTemplateOwner{
				node: dto.KubeGraphNodeDto{
					ID: id, Kind: top.Kind, APIGroup: apiGroup(top.APIVersion), Name: top.Name, Namespace: namespace,
					Status: statusIdle, Drillable: true, Weight: 0,
				},
				labels: pod.Labels,
				spec:   pod.Spec,
			}
			generic[id] = owner
			result.owners = append(result.owners, owner)
		}
		status, _ := podStatus(pod)
		owner.node.Status = worstStatus(owner.node.Status, status)
		owner.node.Weight++
		owner.node.Summary = fmt.Sprintf("%d pods", owner.node.Weight)
		result.podOwner[pod.Name] = id
	}

	return result, nil
}

// listWorkloads registers the built-in workloads drawn at namespace level and
// records the owners of intermediate controllers used to resolve pod ownership.
func (s *kubeTopologyService) listWorkloads(
	ctx context.Context, c *kube.Cluster, namespace string,
	add func(uid types.UID, kind, name, status, summary string, tmpl corev1.PodTemplateSpec) *podTemplateOwner,
	intermediate map[types.UID][]metav1.OwnerReference,
) error {
	apps := c.Clientset.AppsV1()
	batch := c.Clientset.BatchV1()

	deployments, err := apps.Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return kube.WrapError(err, "list deployments")
	}
	for i := range deployments.Items {
		d := &deployments.Items[i]
		status, summary := deploymentStatus(d)
		add(d.UID, "Deployment", d.Name, status, summary, d.Spec.Template)
	}

	statefulSets, err := apps.StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return kube.WrapError(err, "list statefulsets")
	}
	for i := range statefulSets.Items {
		st := &statefulSets.Items[i]
		status, summary := statefulSetStatus(st)
		owner := add(st.UID, "StatefulSet", st.Name, status, summary, st.Spec.Template)
		for _, claim := range st.Spec.VolumeClaimTemplates {
			owner.claimPrefixes = append(owner.claimPrefixes, claim.Name+"-"+st.Name+"-")
		}
	}

	daemonSets, err := apps.DaemonSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return kube.WrapError(err, "list daemonsets")
	}
	for i := range daemonSets.Items {
		d := &daemonSets.Items[i]
		status, summary := daemonSetStatus(d)
		add(d.UID, "DaemonSet", d.Name, status, summary, d.Spec.Template)
	}

	cronJobs, err := batch.CronJobs(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return kube.WrapError(err, "list cronjobs")
	}
	for i := range cronJobs.Items {
		cj := &cronJobs.Items[i]
		status, summary := cronJobStatus(cj)
		add(cj.UID, "CronJob", cj.Name, status, summary, cj.Spec.JobTemplate.Spec.Template)
	}

	jobs, err := batch.Jobs(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return kube.WrapError(err, "list jobs")
	}
	for i := range jobs.Items {
		j := &jobs.Items[i]
		if len(j.OwnerReferences) > 0 {
			intermediate[j.UID] = j.OwnerReferences // e.g. created by a CronJob
			continue
		}
		status, summary := jobStatus(j)
		add(j.UID, "Job", j.Name, status, summary, j.Spec.Template)
	}

	replicaSets, err := apps.ReplicaSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return kube.WrapError(err, "list replicasets")
	}
	for i := range replicaSets.Items {
		intermediate[replicaSets.Items[i].UID] = replicaSets.Items[i].OwnerReferences
	}

	controllers, err := c.Clientset.CoreV1().ReplicationControllers(namespace).List(ctx, metav1.ListOptions{})
	if err != nil && !kube.IsOptional(err) {
		return kube.WrapError(err, "list replicationcontrollers")
	}
	if controllers != nil {
		for i := range controllers.Items {
			rc := &controllers.Items[i]
			status, summary := replicaStatus(rc.Status.ReadyReplicas, derefReplicas(rc.Spec.Replicas))
			if rc.Spec.Template != nil {
				add(rc.UID, "ReplicationController", rc.Name, status, summary, *rc.Spec.Template)
			}
		}
	}
	return nil
}

// topOwner follows controller references up to a drawn workload (returning its node
// ID) or, failing that, to the highest owner reference it can see.
func topOwner(refs []metav1.OwnerReference, shown map[types.UID]string, intermediate map[types.UID][]metav1.OwnerReference) (metav1.OwnerReference, string) {
	ref := controllerRef(refs)
	for depth := 0; depth < maxOwnerDepth; depth++ {
		if id, ok := shown[ref.UID]; ok {
			return ref, id
		}
		parents, ok := intermediate[ref.UID]
		if !ok || len(parents) == 0 {
			return ref, ""
		}
		ref = controllerRef(parents)
	}
	return ref, ""
}

func controllerRef(refs []metav1.OwnerReference) metav1.OwnerReference {
	for _, r := range refs {
		if r.Controller != nil && *r.Controller {
			return r
		}
	}
	return refs[0]
}

func apiGroup(apiVersion string) string {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return ""
	}
	return gv.Group
}
