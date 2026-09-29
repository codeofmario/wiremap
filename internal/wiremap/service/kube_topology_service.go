package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/codeofmario/wiremap/internal/wiremap/dto"
	"github.com/codeofmario/wiremap/internal/wiremap/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
)

// Drill-down levels of the cluster canvas.
const (
	LevelCluster   = "cluster"
	LevelNamespace = "namespace"
	LevelObject    = "object"
	LevelNode      = "node"
)

// Cluster-level lenses.
const (
	LensNamespaces = "namespaces"
	LensNodes      = "nodes"
	LensStorage    = "storage"
	LensAccess     = "access"
)

type KubeTopologyService interface {
	Cluster(ctx context.Context, cluster string, lens string) (*dto.KubeGraphDto, error)
	Namespace(ctx context.Context, cluster string, namespace string) (*dto.KubeGraphDto, error)
	// Object is one object and what it logically contains, e.g. Deployment → ReplicaSets → Pods, Pod → containers.
	Object(ctx context.Context, cluster string, namespace string, group string, kind string, name string) (*dto.KubeGraphDto, error)
	Node(ctx context.Context, cluster string, node string) (*dto.KubeGraphDto, error)
}

type kubeTopologyService struct {
	pool *kube.ClusterPool
}

func NewKubeTopologyService(pool *kube.ClusterPool) KubeTopologyService {
	return &kubeTopologyService{pool: pool}
}

func (s *kubeTopologyService) Cluster(ctx context.Context, clusterName string, lens string) (*dto.KubeGraphDto, error) {
	c, err := s.pool.Get(clusterName)
	if err != nil {
		return nil, err
	}
	switch lens {
	case LensStorage:
		return s.storageGraph(ctx, c)
	case LensAccess:
		return s.accessGraph(ctx, c)
	}

	pods, err := c.Clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, kube.WrapError(err, "list pods")
	}

	if lens == LensNodes {
		return s.nodesGraph(ctx, c, pods.Items)
	}
	return s.namespacesGraph(ctx, c, pods.Items)
}

func (s *kubeTopologyService) namespacesGraph(ctx context.Context, c *kube.Cluster, pods []corev1.Pod) (*dto.KubeGraphDto, error) {
	namespaces, err := c.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, kube.WrapError(err, "list namespaces")
	}

	workloads, err := s.countWorkloads(ctx, c)
	if err != nil {
		return nil, err
	}

	podCount := map[string]int{}
	podHealth := map[string]string{}
	for i := range pods {
		ns := pods[i].Namespace
		status, _ := podStatus(&pods[i])
		podCount[ns]++
		podHealth[ns] = worstStatus(podHealth[ns], status)
	}

	g := newGraph(LevelCluster)
	for _, ns := range namespaces.Items {
		status := podHealth[ns.Name]
		if status == "" {
			status = statusIdle
		}
		if ns.Status.Phase == corev1.NamespaceTerminating {
			status = statusWarning
		}
		g.addNode(dto.KubeGraphNodeDto{
			ID:        kubeNodeID("Namespace", "", ns.Name),
			Kind:      "Namespace",
			Name:      ns.Name,
			Status:    status,
			Summary:   fmt.Sprintf("%d pods · %d workloads", podCount[ns.Name], workloads[ns.Name]),
			Drillable: true,
			Weight:    podCount[ns.Name],
			Details:   map[string]string{"Phase": string(ns.Status.Phase)},
		})
	}
	return g.result(), nil
}

// countWorkloads returns the number of top-level workloads per namespace.
func (s *kubeTopologyService) countWorkloads(ctx context.Context, c *kube.Cluster) (map[string]int, error) {
	counts := map[string]int{}
	apps := c.Clientset.AppsV1()

	deployments, err := apps.Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, kube.WrapError(err, "list deployments")
	}
	for _, d := range deployments.Items {
		counts[d.Namespace]++
	}

	statefulSets, err := apps.StatefulSets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, kube.WrapError(err, "list statefulsets")
	}
	for _, s := range statefulSets.Items {
		counts[s.Namespace]++
	}

	daemonSets, err := apps.DaemonSets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, kube.WrapError(err, "list daemonsets")
	}
	for _, d := range daemonSets.Items {
		counts[d.Namespace]++
	}

	cronJobs, err := c.Clientset.BatchV1().CronJobs("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, kube.WrapError(err, "list cronjobs")
	}
	for _, cj := range cronJobs.Items {
		counts[cj.Namespace]++
	}

	return counts, nil
}

func (s *kubeTopologyService) nodesGraph(ctx context.Context, c *kube.Cluster, pods []corev1.Pod) (*dto.KubeGraphDto, error) {
	nodes, err := c.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, kube.WrapError(err, "list nodes")
	}

	podCount := map[string]int{}
	for _, p := range pods {
		podCount[p.Spec.NodeName]++
	}

	g := newGraph(LevelCluster)
	for i := range nodes.Items {
		n := &nodes.Items[i]
		g.addNode(dto.KubeGraphNodeDto{
			ID:        kubeNodeID("Node", "", n.Name),
			Kind:      "Node",
			Name:      n.Name,
			Status:    nodeStatus(n),
			Summary:   fmt.Sprintf("%d pods · %s", podCount[n.Name], n.Status.NodeInfo.KubeletVersion),
			Drillable: true,
			Weight:    podCount[n.Name],
			Details: map[string]string{
				"Roles":  nodeRoles(n),
				"OS":     n.Status.NodeInfo.OSImage,
				"CPU":    n.Status.Capacity.Cpu().String(),
				"Memory": n.Status.Capacity.Memory().String(),
			},
		})
	}
	return g.result(), nil
}

func (s *kubeTopologyService) Node(ctx context.Context, clusterName string, nodeName string) (*dto.KubeGraphDto, error) {
	c, err := s.pool.Get(clusterName)
	if err != nil {
		return nil, err
	}

	pods, err := c.Clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("spec.nodeName", nodeName).String(),
	})
	if err != nil {
		return nil, kube.WrapError(err, "list pods on node")
	}

	g := newGraph(LevelNode)
	for i := range pods.Items {
		pod := &pods.Items[i]
		groupID := kubeNodeID("Namespace", "", pod.Namespace)
		g.addGroup(groupID, pod.Namespace, "Namespace")
		g.addNode(podNode(pod, groupID))
	}
	return g.result(), nil
}

func nodeRoles(n *corev1.Node) string {
	var roles []string
	for label := range n.Labels {
		if role, ok := strings.CutPrefix(label, "node-role.kubernetes.io/"); ok {
			roles = append(roles, role)
		}
	}
	if len(roles) == 0 {
		return "worker"
	}
	sort.Strings(roles)
	return strings.Join(roles, ", ")
}

func podNode(pod *corev1.Pod, group string) dto.KubeGraphNodeDto {
	status, summary := podStatus(pod)
	return dto.KubeGraphNodeDto{
		ID:        kubeNodeID("Pod", pod.Namespace, pod.Name),
		Kind:      "Pod",
		Name:      pod.Name,
		Namespace: pod.Namespace,
		Status:    status,
		Summary:   summary,
		Drillable: true,
		Group:     group,
		Weight:    1,
		Details: map[string]string{
			"Node":   pod.Spec.NodeName,
			"Pod IP": pod.Status.PodIP,
			"Images": containerImages(pod.Spec),
		},
	}
}

func kubeNodeID(kind, namespace, name string) string {
	return kind + "/" + namespace + "/" + name
}

// graphBuilder accumulates a graph level, skipping duplicate nodes, edges and groups.
type graphBuilder struct {
	graph  dto.KubeGraphDto
	nodes  map[string]bool
	edges  map[dto.KubeGraphEdgeDto]bool
	groups map[string]bool
}

func newGraph(level string) *graphBuilder {
	return &graphBuilder{
		graph: dto.KubeGraphDto{
			Level:  level,
			Nodes:  []dto.KubeGraphNodeDto{},
			Edges:  []dto.KubeGraphEdgeDto{},
			Groups: []dto.KubeGraphGroupDto{},
		},
		nodes:  map[string]bool{},
		edges:  map[dto.KubeGraphEdgeDto]bool{},
		groups: map[string]bool{},
	}
}

func (g *graphBuilder) addNode(node dto.KubeGraphNodeDto) {
	if g.nodes[node.ID] {
		return
	}
	if node.APIGroup == "" {
		node.APIGroup = kindGroups[node.Kind]
	}
	g.nodes[node.ID] = true
	g.graph.Nodes = append(g.graph.Nodes, node)
}

func (g *graphBuilder) hasNode(id string) bool {
	return g.nodes[id]
}

func (g *graphBuilder) addEdge(source, target, kind string) {
	edge := dto.KubeGraphEdgeDto{Source: source, Target: target, Kind: kind}
	if g.edges[edge] {
		return
	}
	g.edges[edge] = true
	g.graph.Edges = append(g.graph.Edges, edge)
}

func (g *graphBuilder) addGroup(id, name, kind string) {
	if g.groups[id] {
		return
	}
	g.groups[id] = true
	g.graph.Groups = append(g.graph.Groups, dto.KubeGraphGroupDto{ID: id, Name: name, Kind: kind})
}

func (g *graphBuilder) result() *dto.KubeGraphDto {
	return &g.graph
}
