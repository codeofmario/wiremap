package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/codeofmario/wiremap/internal/wiremap/dto"
	"github.com/codeofmario/wiremap/internal/wiremap/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// addPodContainers draws a pod → its init and app containers.
func (s *kubeTopologyService) addPodContainers(ctx context.Context, c *kube.Cluster, namespace, name string, g *graphBuilder) error {
	pod, err := c.Clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return kube.WrapError(err, "get pod")
	}

	root := podNode(pod, "")
	root.Drillable, root.Weight = false, 2
	g.addNode(root)

	containers := append(
		containerDtos(pod.Spec.InitContainers, pod.Status.InitContainerStatuses, true),
		containerDtos(pod.Spec.Containers, pod.Status.ContainerStatuses, false)...,
	)
	specs := append(append([]corev1.Container{}, pod.Spec.InitContainers...), pod.Spec.Containers...)

	for i, ctr := range containers {
		id := kubeNodeID("Container", namespace, pod.Name+"/"+ctr.Name)
		g.addNode(containerNode(id, namespace, ctr, specs[i]))
		g.addEdge(root.ID, id, "owns")
	}
	return nil
}

func containerNode(id, namespace string, ctr dto.KubeContainerDto, spec corev1.Container) dto.KubeGraphNodeDto {
	status := containerStatus(ctr)
	summary := ctr.State
	if ctr.Reason != "" {
		summary = ctr.Reason
	}
	if ctr.Init {
		summary = "init · " + summary
	}
	if ctr.RestartCount > 0 {
		summary += fmt.Sprintf(" · %d restarts", ctr.RestartCount)
	}

	ports := make([]string, 0, len(spec.Ports))
	for _, p := range spec.Ports {
		ports = append(ports, fmt.Sprintf("%d/%s", p.ContainerPort, p.Protocol))
	}
	return dto.KubeGraphNodeDto{
		ID: id, Kind: "Container", Name: ctr.Name, Namespace: namespace,
		Status: status, Summary: summary, Drillable: true, Weight: 1,
		Details: map[string]string{
			"Image":   ctr.Image,
			"Ports":   strings.Join(ports, ", "),
			"Ready":   fmt.Sprintf("%t", ctr.Ready),
			"Command": strings.Join(append(spec.Command, spec.Args...), " "),
		},
	}
}

func containerStatus(ctr dto.KubeContainerDto) string {
	switch ctr.State {
	case "running":
		if ctr.Ready || ctr.Init {
			return statusHealthy
		}
		return statusWarning
	case "terminated":
		if ctr.Reason == "Completed" {
			return statusIdle
		}
		return statusError
	}
	if ctr.Reason == "" || benignWaitingReasons[ctr.Reason] {
		return statusWarning
	}
	return statusError
}

// addServiceBackends draws a Service → the pods matching its selector or, for
// selector-less Services, the pods in its EndpointSlices.
func (s *kubeTopologyService) addServiceBackends(ctx context.Context, c *kube.Cluster, namespace, name string, g *graphBuilder) error {
	svc, err := c.Clientset.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return kube.WrapError(err, "get service")
	}
	pods, err := c.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return kube.WrapError(err, "list pods")
	}

	root := serviceNode(svc)
	root.Drillable, root.Weight = false, 2
	g.addNode(root)

	var endpoints map[string]bool
	if len(svc.Spec.Selector) == 0 {
		endpoints = map[string]bool{}
		for _, pod := range s.endpointPods(ctx, c, namespace)[name] {
			endpoints[pod] = true
		}
	}
	selector := labels.SelectorFromSet(svc.Spec.Selector)

	for i := range pods.Items {
		pod := &pods.Items[i]
		selected := endpoints[pod.Name]
		if endpoints == nil {
			selected = selector.Matches(labels.Set(pod.Labels))
		}
		if selected {
			g.addEdge(root.ID, addScheduledPod(g, pod), "selects")
		}
	}
	return nil
}

// addIngressBackends draws an Ingress → the Services it routes to.
func (s *kubeTopologyService) addIngressBackends(ctx context.Context, c *kube.Cluster, namespace, name string, g *graphBuilder) error {
	ing, err := c.Clientset.NetworkingV1().Ingresses(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return kube.WrapError(err, "get ingress")
	}
	rootID := kubeNodeID("Ingress", namespace, name)
	g.addNode(dto.KubeGraphNodeDto{
		ID: rootID, Kind: "Ingress", Name: name, Namespace: namespace,
		Status: statusHealthy, Summary: strings.Join(ingressHosts(ing), ", "), Weight: 2,
	})

	for _, backend := range ingressBackends(ing) {
		svc, err := c.Clientset.CoreV1().Services(namespace).Get(ctx, backend, metav1.GetOptions{})
		if err != nil {
			id := kubeNodeID("Service", namespace, backend)
			g.addNode(dto.KubeGraphNodeDto{ID: id, Kind: "Service", Name: backend, Namespace: namespace, Status: statusWarning, Summary: "missing", Weight: 1})
			g.addEdge(rootID, id, "routes")
			continue
		}
		node := serviceNode(svc)
		g.addNode(node)
		g.addEdge(rootID, node.ID, "routes")
	}
	return nil
}

// addConsumers draws the pods that mount or read a ConfigMap, Secret or PVC,
// and for a PVC the volume and storage class behind it.
func (s *kubeTopologyService) addConsumers(ctx context.Context, c *kube.Cluster, namespace, kind, name string, g *graphBuilder) error {
	obj, err := getObject(ctx, c, "", kind, namespace, name)
	if err != nil {
		return err
	}
	pods, err := c.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return kube.WrapError(err, "list pods")
	}

	status, summary := objectStatus(obj)
	rootID := kubeNodeID(kind, namespace, name)
	g.addNode(dto.KubeGraphNodeDto{
		ID: rootID, Kind: kind, Name: name, Namespace: namespace,
		Status: status, Summary: summary, Weight: 2,
	})

	target := specRef{kind, name}
	for i := range pods.Items {
		pod := &pods.Items[i]
		for _, ref := range podSpecRefs(pod.Spec) {
			if ref == target {
				g.addEdge(addScheduledPod(g, pod), rootID, "uses")
				break
			}
		}
	}

	if kind == "PersistentVolumeClaim" {
		s.addClaimVolume(ctx, c, namespace, name, g)
	}
	return nil
}

// addClaimVolume draws PVC → PersistentVolume → StorageClass, skipping what RBAC hides.
func (s *kubeTopologyService) addClaimVolume(ctx context.Context, c *kube.Cluster, namespace, name string, g *graphBuilder) {
	pvc, err := c.Clientset.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return
	}
	claimID := kubeNodeID("PersistentVolumeClaim", namespace, name)
	if pvc.Spec.VolumeName != "" {
		if pv, err := c.Clientset.CoreV1().PersistentVolumes().Get(ctx, pvc.Spec.VolumeName, metav1.GetOptions{}); err == nil {
			pvID := addPersistentVolume(g, pv)
			g.addEdge(claimID, pvID, "bound")
			if pv.Spec.StorageClassName != "" {
				g.addEdge(pvID, addStorageClass(g, pv.Spec.StorageClassName), "uses")
			}
			return
		}
	}
	if pvc.Spec.StorageClassName != nil && *pvc.Spec.StorageClassName != "" {
		g.addEdge(claimID, addStorageClass(g, *pvc.Spec.StorageClassName), "uses")
	}
}
