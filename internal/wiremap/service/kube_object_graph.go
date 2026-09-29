package service

import (
	"context"

	"github.com/codeofmario/wiremap/internal/wiremap/dto"
	"github.com/codeofmario/wiremap/internal/wiremap/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const unscheduledGroup = "unscheduled"

func (s *kubeTopologyService) Object(ctx context.Context, clusterName string, namespace string, group string, kind string, name string) (*dto.KubeGraphDto, error) {
	c, err := s.pool.Get(clusterName)
	if err != nil {
		return nil, err
	}

	g := newGraph(LevelObject)
	switch group + "/" + kind {
	case "/Pod":
		err = s.addPodContainers(ctx, c, namespace, name, g)
	case "/Service":
		err = s.addServiceBackends(ctx, c, namespace, name, g)
	case "networking.k8s.io/Ingress":
		err = s.addIngressBackends(ctx, c, namespace, name, g)
	case "/ConfigMap", "/Secret", "/PersistentVolumeClaim":
		err = s.addConsumers(ctx, c, namespace, kind, name, g)
	default:
		err = s.addOwnerTree(ctx, c, namespace, group, kind, name, g)
	}
	if err != nil {
		return nil, err
	}
	return g.result(), nil
}

// addOwnerTree draws an owner → its pods grouped by node, or a CronJob → its Jobs.
func (s *kubeTopologyService) addOwnerTree(ctx context.Context, c *kube.Cluster, namespace, group, kind, name string, g *graphBuilder) error {
	// Graph node IDs of the direct pod owners, keyed by UID
	podParents := map[types.UID]string{}

	var err error
	switch group + "/" + kind {
	case "apps/Deployment":
		err = s.addDeployment(ctx, c, namespace, name, g, podParents)
	case "apps/StatefulSet":
		err = s.addStatefulSet(ctx, c, namespace, name, g, podParents)
	case "apps/DaemonSet":
		err = s.addDaemonSet(ctx, c, namespace, name, g, podParents)
	case "batch/Job":
		err = s.addJob(ctx, c, namespace, name, g, podParents)
	case "batch/CronJob":
		err = s.addCronJob(ctx, c, namespace, name, g, podParents)
	default:
		// ReplicaSets, ReplicationControllers and custom controllers (e.g. Argo Rollouts)
		err = s.addGenericOwner(ctx, c, namespace, group, kind, name, g, podParents)
	}
	if err != nil {
		return err
	}
	return s.addOwnedPods(ctx, c, namespace, g, podParents)
}

func (s *kubeTopologyService) addDeployment(ctx context.Context, c *kube.Cluster, namespace, name string, g *graphBuilder, podParents map[types.UID]string) error {
	d, err := c.Clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return kube.WrapError(err, "get deployment")
	}
	status, summary := deploymentStatus(d)
	rootID := addWorkloadRoot(g, "Deployment", namespace, name, status, summary)

	return s.collapseReplicaSets(ctx, c, namespace, d.UID, rootID, podParents)
}

// collapseReplicaSets attributes the pods of an owner's ReplicaSets to the owner
// itself, so drilling in goes straight to pods.
func (s *kubeTopologyService) collapseReplicaSets(ctx context.Context, c *kube.Cluster, namespace string, owner types.UID, ownerID string, podParents map[types.UID]string) error {
	replicaSets, err := c.Clientset.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return kube.WrapError(err, "list replicasets")
	}
	for i := range replicaSets.Items {
		if rs := &replicaSets.Items[i]; ownedBy(rs.OwnerReferences, owner) {
			podParents[rs.UID] = ownerID
		}
	}
	return nil
}

func (s *kubeTopologyService) addStatefulSet(ctx context.Context, c *kube.Cluster, namespace, name string, g *graphBuilder, podParents map[types.UID]string) error {
	st, err := c.Clientset.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return kube.WrapError(err, "get statefulset")
	}
	status, summary := statefulSetStatus(st)
	podParents[st.UID] = addWorkloadRoot(g, "StatefulSet", namespace, name, status, summary)
	return nil
}

func (s *kubeTopologyService) addDaemonSet(ctx context.Context, c *kube.Cluster, namespace, name string, g *graphBuilder, podParents map[types.UID]string) error {
	d, err := c.Clientset.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return kube.WrapError(err, "get daemonset")
	}
	status, summary := daemonSetStatus(d)
	podParents[d.UID] = addWorkloadRoot(g, "DaemonSet", namespace, name, status, summary)
	return nil
}

func (s *kubeTopologyService) addJob(ctx context.Context, c *kube.Cluster, namespace, name string, g *graphBuilder, podParents map[types.UID]string) error {
	j, err := c.Clientset.BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return kube.WrapError(err, "get job")
	}
	status, summary := jobStatus(j)
	podParents[j.UID] = addWorkloadRoot(g, "Job", namespace, name, status, summary)
	return nil
}

func (s *kubeTopologyService) addCronJob(ctx context.Context, c *kube.Cluster, namespace, name string, g *graphBuilder, podParents map[types.UID]string) error {
	cj, err := c.Clientset.BatchV1().CronJobs(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return kube.WrapError(err, "get cronjob")
	}
	status, summary := cronJobStatus(cj)
	rootID := addWorkloadRoot(g, "CronJob", namespace, name, status, summary)

	jobs, err := c.Clientset.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return kube.WrapError(err, "list jobs")
	}
	for i := range jobs.Items {
		j := &jobs.Items[i]
		if !ownedBy(j.OwnerReferences, cj.UID) {
			continue
		}
		jStatus, jSummary := jobStatus(j)
		id := kubeNodeID("Job", namespace, j.Name)
		g.addNode(dto.KubeGraphNodeDto{
			ID: id, Kind: "Job", Name: j.Name, Namespace: namespace,
			Status: jStatus, Summary: jSummary, Drillable: true, Weight: 1,
		})
		g.addEdge(rootID, id, "owns")
	}
	return nil
}

// addOwnedPods adds the pods owned by podParents, grouped by the node they run on.
func (s *kubeTopologyService) addOwnedPods(ctx context.Context, c *kube.Cluster, namespace string, g *graphBuilder, podParents map[types.UID]string) error {
	pods, err := c.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return kube.WrapError(err, "list pods")
	}

	for i := range pods.Items {
		pod := &pods.Items[i]
		parentID := ""
		for _, ref := range pod.OwnerReferences {
			if id, ok := podParents[ref.UID]; ok {
				parentID = id
				break
			}
		}
		if parentID == "" {
			continue
		}

		g.addEdge(parentID, addScheduledPod(g, pod), "owns")
	}
	return nil
}

// addScheduledPod adds a pod grouped by the node it runs on and returns its graph node ID.
func addScheduledPod(g *graphBuilder, pod *corev1.Pod) string {
	nodeName := pod.Spec.NodeName
	if nodeName == "" {
		nodeName = unscheduledGroup
	}
	groupID := kubeNodeID("Node", "", nodeName)
	g.addGroup(groupID, nodeName, "Node")

	node := podNode(pod, groupID)
	g.addNode(node)
	return node.ID
}

// addGenericOwner draws any object that owns pods, directly or through the
// ReplicaSets it creates, and the Jobs it creates.
func (s *kubeTopologyService) addGenericOwner(ctx context.Context, c *kube.Cluster, namespace, group, kind, name string, g *graphBuilder, podParents map[types.UID]string) error {
	obj, err := getObject(ctx, c, group, kind, namespace, name)
	if err != nil {
		return err
	}
	status, summary := objectStatus(obj)
	rootID := kubeNodeID(kind, namespace, name)
	g.addNode(dto.KubeGraphNodeDto{
		ID: rootID, Kind: kind, APIGroup: group, Name: name, Namespace: namespace,
		Status: status, Summary: summary, Weight: 2,
	})
	podParents[obj.GetUID()] = rootID

	if err := s.collapseReplicaSets(ctx, c, namespace, obj.GetUID(), rootID, podParents); err != nil {
		return err
	}

	jobs, err := c.Clientset.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return kube.WrapError(err, "list jobs")
	}
	for i := range jobs.Items {
		j := &jobs.Items[i]
		if !ownedBy(j.OwnerReferences, obj.GetUID()) {
			continue
		}
		jStatus, jSummary := jobStatus(j)
		id := kubeNodeID("Job", namespace, j.Name)
		g.addNode(dto.KubeGraphNodeDto{ID: id, Kind: "Job", Name: j.Name, Namespace: namespace, Status: jStatus, Summary: jSummary, Drillable: true, Weight: 1})
		g.addEdge(rootID, id, "owns")
	}
	return nil
}

func addWorkloadRoot(g *graphBuilder, kind, namespace, name, status, summary string) string {
	id := kubeNodeID(kind, namespace, name)
	g.addNode(dto.KubeGraphNodeDto{
		ID: id, Kind: kind, Name: name, Namespace: namespace,
		Status: status, Summary: summary, Weight: 2,
	})
	return id
}

func ownedBy(refs []metav1.OwnerReference, uid types.UID) bool {
	for _, r := range refs {
		if r.UID == uid {
			return true
		}
	}
	return false
}
