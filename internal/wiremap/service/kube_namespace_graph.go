package service

import (
	"context"
	"strings"

	"github.com/codeofmario/wiremap/internal/wiremap/dto"
	"github.com/codeofmario/wiremap/internal/wiremap/kube"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// namespaceStep adds one family of objects to the namespace graph.
type namespaceStep func(ctx context.Context, c *kube.Cluster, namespace string, owners *namespaceOwners, g *graphBuilder) error

func (s *kubeTopologyService) Namespace(ctx context.Context, clusterName string, namespace string) (*dto.KubeGraphDto, error) {
	c, err := s.pool.Get(clusterName)
	if err != nil {
		return nil, err
	}

	owners, err := s.listPodOwners(ctx, c, namespace)
	if err != nil {
		return nil, err
	}

	g := newGraph(LevelNamespace)
	for _, o := range owners.owners {
		g.addNode(o.node)
	}

	// Order matters: later steps link to nodes added by earlier ones
	steps := []namespaceStep{
		s.addServices,
		s.addIngresses,
		s.addGatewayRoutes,
		s.addConfigRefs,
		s.addAutoscalers,
		s.addDisruptionBudgets,
		s.addNetworkPolicies,
		s.addServiceAccounts,
		s.addQuotas,
	}
	for _, step := range steps {
		if err := step(ctx, c, namespace, owners, g); err != nil {
			return nil, err
		}
	}

	return g.result(), nil
}

// addServices links each Service to the workloads whose pod labels match its
// selector, or — for selector-less Services — to the pods in its EndpointSlices.
func (s *kubeTopologyService) addServices(ctx context.Context, c *kube.Cluster, namespace string, owners *namespaceOwners, g *graphBuilder) error {
	services, err := c.Clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return kube.WrapError(err, "list services")
	}

	var endpointPods map[string][]string
	for i := range services.Items {
		svc := &services.Items[i]
		node := serviceNode(svc)
		id := node.ID
		g.addNode(node)

		if len(svc.Spec.Selector) > 0 {
			selector := labels.SelectorFromSet(svc.Spec.Selector)
			for _, o := range owners.owners {
				if selector.Matches(labels.Set(o.labels)) {
					g.addEdge(id, o.node.ID, "selects")
				}
			}
			continue
		}

		if endpointPods == nil {
			endpointPods = s.endpointPods(ctx, c, namespace)
		}
		for _, pod := range endpointPods[svc.Name] {
			if ownerID, ok := owners.podOwner[pod]; ok {
				g.addEdge(id, ownerID, "selects")
			}
		}
	}
	return nil
}

func serviceNode(svc *corev1.Service) dto.KubeGraphNodeDto {
	return dto.KubeGraphNodeDto{
		ID:        kubeNodeID("Service", svc.Namespace, svc.Name),
		Kind:      "Service",
		Name:      svc.Name,
		Namespace: svc.Namespace,
		Status:    statusHealthy,
		Summary:   serviceSummary(svc),
		Drillable: true,
		Weight:    1,
		Details:   map[string]string{"Cluster IP": svc.Spec.ClusterIP},
	}
}

// endpointPods returns the pod names behind each Service, from its EndpointSlices.
func (s *kubeTopologyService) endpointPods(ctx context.Context, c *kube.Cluster, namespace string) map[string][]string {
	result := map[string][]string{}
	slices, err := c.Clientset.DiscoveryV1().EndpointSlices(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return result // optional: links are a bonus for selector-less Services
	}
	for _, slice := range slices.Items {
		service := slice.Labels[discoveryv1.LabelServiceName]
		for _, ep := range slice.Endpoints {
			if ep.TargetRef != nil && ep.TargetRef.Kind == "Pod" {
				result[service] = append(result[service], ep.TargetRef.Name)
			}
		}
	}
	return result
}

// addIngresses links each Ingress to the Services it routes to.
func (s *kubeTopologyService) addIngresses(ctx context.Context, c *kube.Cluster, namespace string, _ *namespaceOwners, g *graphBuilder) error {
	ingresses, err := c.Clientset.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return kube.WrapError(err, "list ingresses")
	}

	for i := range ingresses.Items {
		ing := &ingresses.Items[i]
		id := kubeNodeID("Ingress", namespace, ing.Name)
		details := map[string]string{}
		if ing.Spec.IngressClassName != nil {
			details["Class"] = *ing.Spec.IngressClassName
		}
		g.addNode(dto.KubeGraphNodeDto{
			ID:        id,
			Kind:      "Ingress",
			Name:      ing.Name,
			Namespace: namespace,
			Status:    statusHealthy,
			Summary:   strings.Join(ingressHosts(ing), ", "),
			Drillable: true,
			Weight:    1,
			Details:   details,
		})

		for _, backend := range ingressBackends(ing) {
			target := kubeNodeID("Service", namespace, backend)
			if g.hasNode(target) {
				g.addEdge(id, target, "routes")
			}
		}
	}
	return nil
}

// addConfigRefs adds the ConfigMaps, Secrets and PVCs pods depend on, and the
// PersistentVolumes and StorageClasses behind those PVCs.
func (s *kubeTopologyService) addConfigRefs(ctx context.Context, c *kube.Cluster, namespace string, owners *namespaceOwners, g *graphBuilder) error {
	pvcs, err := c.Clientset.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return kube.WrapError(err, "list persistentvolumeclaims")
	}
	pvcByName := map[string]*corev1.PersistentVolumeClaim{}
	for i := range pvcs.Items {
		pvcByName[pvcs.Items[i].Name] = &pvcs.Items[i]
	}

	usedClaims := map[string]bool{}
	for _, o := range owners.owners {
		refs := podSpecRefs(o.spec)
		for _, prefix := range o.claimPrefixes {
			for name := range pvcByName {
				if strings.HasPrefix(name, prefix) {
					refs = append(refs, specRef{"PersistentVolumeClaim", name})
				}
			}
		}

		for _, ref := range refs {
			id := kubeNodeID(ref.kind, namespace, ref.name)
			node := dto.KubeGraphNodeDto{
				ID: id, Kind: ref.kind, Name: ref.name, Namespace: namespace,
				Status: statusHealthy, Drillable: true, Weight: 1,
			}
			if ref.kind == "PersistentVolumeClaim" {
				usedClaims[ref.name] = true
				node.Status, node.Summary = statusWarning, "missing"
				if pvc, ok := pvcByName[ref.name]; ok {
					node.Status, node.Summary = pvcStatus(pvc)
				}
			}
			g.addNode(node)
			g.addEdge(o.node.ID, id, "uses")
		}
	}

	s.addVolumes(ctx, c, namespace, pvcByName, usedClaims, g)
	return nil
}

// addVolumes draws PVC → PersistentVolume → StorageClass for the claims in use.
// Both are cluster-scoped, so they are skipped when RBAC doesn't allow reading them.
func (s *kubeTopologyService) addVolumes(ctx context.Context, c *kube.Cluster, namespace string, pvcByName map[string]*corev1.PersistentVolumeClaim, used map[string]bool, g *graphBuilder) {
	pvs, err := c.Clientset.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		pvs = &corev1.PersistentVolumeList{}
	}
	pvByName := map[string]*corev1.PersistentVolume{}
	for i := range pvs.Items {
		pvByName[pvs.Items[i].Name] = &pvs.Items[i]
	}

	for name := range used {
		pvc, ok := pvcByName[name]
		if !ok {
			continue
		}
		claimID := kubeNodeID("PersistentVolumeClaim", namespace, name)
		className := ""
		if pvc.Spec.StorageClassName != nil {
			className = *pvc.Spec.StorageClassName
		}

		if pv, ok := pvByName[pvc.Spec.VolumeName]; ok {
			pvID := addPersistentVolume(g, pv)
			g.addEdge(claimID, pvID, "bound")
			if pv.Spec.StorageClassName != "" {
				g.addEdge(pvID, addStorageClass(g, pv.Spec.StorageClassName), "uses")
			}
		} else if className != "" {
			g.addEdge(claimID, addStorageClass(g, className), "uses")
		}
	}
}

func addPersistentVolume(g *graphBuilder, pv *corev1.PersistentVolume) string {
	id := kubeNodeID("PersistentVolume", "", pv.Name)
	status, _ := phaseStatus(string(pv.Status.Phase))
	size := pv.Spec.Capacity[corev1.ResourceStorage]
	g.addNode(dto.KubeGraphNodeDto{
		ID: id, Kind: "PersistentVolume", Name: pv.Name, Status: status, Weight: 1,
		Summary: string(pv.Status.Phase) + " · " + size.String(),
		Details: map[string]string{"Reclaim policy": string(pv.Spec.PersistentVolumeReclaimPolicy)},
	})
	return id
}

func addStorageClass(g *graphBuilder, name string) string {
	id := kubeNodeID("StorageClass", "", name)
	g.addNode(dto.KubeGraphNodeDto{ID: id, Kind: "StorageClass", Name: name, Status: statusHealthy, Weight: 1})
	return id
}

func phaseStatus(phase string) (string, string) {
	if status, ok := phaseStatuses[strings.ToLower(phase)]; ok {
		return status, phase
	}
	return statusIdle, phase
}

type specRef struct {
	kind string
	name string
}

// podSpecRefs lists the distinct ConfigMaps, Secrets and PVCs a pod spec depends on.
func podSpecRefs(spec corev1.PodSpec) []specRef {
	seen := map[specRef]bool{}
	var refs []specRef
	add := func(kind, name string) {
		ref := specRef{kind, name}
		if name != "" && !seen[ref] {
			seen[ref] = true
			refs = append(refs, ref)
		}
	}

	for _, v := range spec.Volumes {
		switch {
		case v.ConfigMap != nil:
			add("ConfigMap", v.ConfigMap.Name)
		case v.Secret != nil:
			add("Secret", v.Secret.SecretName)
		case v.PersistentVolumeClaim != nil:
			add("PersistentVolumeClaim", v.PersistentVolumeClaim.ClaimName)
		case v.Projected != nil:
			for _, src := range v.Projected.Sources {
				if src.ConfigMap != nil {
					add("ConfigMap", src.ConfigMap.Name)
				}
				if src.Secret != nil {
					add("Secret", src.Secret.Name)
				}
			}
		}
	}

	for _, secret := range spec.ImagePullSecrets {
		add("Secret", secret.Name)
	}

	containers := append(append([]corev1.Container{}, spec.InitContainers...), spec.Containers...)
	for _, ctr := range containers {
		for _, from := range ctr.EnvFrom {
			if from.ConfigMapRef != nil {
				add("ConfigMap", from.ConfigMapRef.Name)
			}
			if from.SecretRef != nil {
				add("Secret", from.SecretRef.Name)
			}
		}
		for _, env := range ctr.Env {
			if env.ValueFrom == nil {
				continue
			}
			if ref := env.ValueFrom.ConfigMapKeyRef; ref != nil {
				add("ConfigMap", ref.Name)
			}
			if ref := env.ValueFrom.SecretKeyRef; ref != nil {
				add("Secret", ref.Name)
			}
		}
	}
	return refs
}

func ingressHosts(ing *networkingv1.Ingress) []string {
	var hosts []string
	for _, rule := range ing.Spec.Rules {
		if rule.Host != "" {
			hosts = append(hosts, rule.Host)
		}
	}
	if len(hosts) == 0 {
		hosts = append(hosts, "*")
	}
	return hosts
}

func ingressBackends(ing *networkingv1.Ingress) []string {
	var names []string
	if b := ing.Spec.DefaultBackend; b != nil && b.Service != nil {
		names = append(names, b.Service.Name)
	}
	for _, rule := range ing.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		for _, path := range rule.HTTP.Paths {
			if path.Backend.Service != nil {
				names = append(names, path.Backend.Service.Name)
			}
		}
	}
	return names
}
