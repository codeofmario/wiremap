package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/codeofmario/wiremap/internal/wiremap/dto"
	"github.com/codeofmario/wiremap/internal/wiremap/kube"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// The steps in this file are optional: RBAC or older clusters may not allow
// them, which hides that family of objects instead of failing the whole view.

// addAutoscalers links each HorizontalPodAutoscaler to the workload it scales.
func (s *kubeTopologyService) addAutoscalers(ctx context.Context, c *kube.Cluster, namespace string, _ *namespaceOwners, g *graphBuilder) error {
	hpas, err := c.Clientset.AutoscalingV2().HorizontalPodAutoscalers(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return optionalStep(err, "list horizontalpodautoscalers")
	}

	for i := range hpas.Items {
		hpa := &hpas.Items[i]
		id := kubeNodeID("HorizontalPodAutoscaler", namespace, hpa.Name)
		minReplicas := int32(1)
		if hpa.Spec.MinReplicas != nil {
			minReplicas = *hpa.Spec.MinReplicas
		}
		summary := fmt.Sprintf("%d–%d replicas · now %d", minReplicas, hpa.Spec.MaxReplicas, hpa.Status.CurrentReplicas)
		g.addNode(dto.KubeGraphNodeDto{
			ID: id, Kind: "HorizontalPodAutoscaler", Name: hpa.Name, Namespace: namespace,
			Status: autoscalerStatus(hpa), Summary: summary, Weight: 1,
			Details: map[string]string{"Targets": autoscalerTargets(hpa)},
		})

		target := kubeNodeID(hpa.Spec.ScaleTargetRef.Kind, namespace, hpa.Spec.ScaleTargetRef.Name)
		if g.hasNode(target) {
			g.addEdge(id, target, "scales")
		}
	}
	return nil
}

func autoscalerStatus(hpa *autoscalingv2.HorizontalPodAutoscaler) string {
	status := statusHealthy
	for _, cond := range hpa.Status.Conditions {
		if cond.Status != corev1.ConditionFalse {
			continue
		}
		switch cond.Type {
		case autoscalingv2.AbleToScale:
			return statusError
		case autoscalingv2.ScalingActive:
			status = statusWarning
		}
	}
	return status
}

func autoscalerTargets(hpa *autoscalingv2.HorizontalPodAutoscaler) string {
	var targets []string
	for _, m := range hpa.Spec.Metrics {
		if m.Resource != nil && m.Resource.Target.AverageUtilization != nil {
			targets = append(targets, fmt.Sprintf("%s %d%%", m.Resource.Name, *m.Resource.Target.AverageUtilization))
		} else {
			targets = append(targets, string(m.Type))
		}
	}
	return strings.Join(targets, ", ")
}

// addDisruptionBudgets links each PodDisruptionBudget to the workloads it protects.
func (s *kubeTopologyService) addDisruptionBudgets(ctx context.Context, c *kube.Cluster, namespace string, owners *namespaceOwners, g *graphBuilder) error {
	pdbs, err := c.Clientset.PolicyV1().PodDisruptionBudgets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return optionalStep(err, "list poddisruptionbudgets")
	}

	for i := range pdbs.Items {
		pdb := &pdbs.Items[i]
		id := kubeNodeID("PodDisruptionBudget", namespace, pdb.Name)
		rule := ""
		switch {
		case pdb.Spec.MinAvailable != nil:
			rule = "min available " + pdb.Spec.MinAvailable.String()
		case pdb.Spec.MaxUnavailable != nil:
			rule = "max unavailable " + pdb.Spec.MaxUnavailable.String()
		}
		status := statusHealthy
		if pdb.Status.ExpectedPods > 0 && pdb.Status.DisruptionsAllowed == 0 {
			status = statusWarning
		}
		g.addNode(dto.KubeGraphNodeDto{
			ID: id, Kind: "PodDisruptionBudget", Name: pdb.Name, Namespace: namespace, Status: status, Weight: 1,
			Summary: fmt.Sprintf("%s · %d disruptions allowed", rule, pdb.Status.DisruptionsAllowed),
		})

		selector, err := metav1.LabelSelectorAsSelector(pdb.Spec.Selector)
		if err != nil {
			continue
		}
		for _, o := range matchingOwners(owners, selector) {
			g.addEdge(id, o.node.ID, "protects")
		}
	}
	return nil
}

// addNetworkPolicies links each NetworkPolicy to the workloads it applies to, and
// draws "allows" edges between workloads its pod-selector rules connect.
func (s *kubeTopologyService) addNetworkPolicies(ctx context.Context, c *kube.Cluster, namespace string, owners *namespaceOwners, g *graphBuilder) error {
	policies, err := c.Clientset.NetworkingV1().NetworkPolicies(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return optionalStep(err, "list networkpolicies")
	}

	for i := range policies.Items {
		policy := &policies.Items[i]
		id := kubeNodeID("NetworkPolicy", namespace, policy.Name)
		external := 0

		selector, err := metav1.LabelSelectorAsSelector(&policy.Spec.PodSelector)
		if err != nil {
			continue
		}
		targets := matchingOwners(owners, selector)

		peerOwners := func(peers []networkingv1.NetworkPolicyPeer) []*podTemplateOwner {
			var result []*podTemplateOwner
			for _, peer := range peers {
				if peer.PodSelector == nil || peer.NamespaceSelector != nil {
					external++ // other namespaces and CIDRs aren't drawn at this level
					continue
				}
				if sel, err := metav1.LabelSelectorAsSelector(peer.PodSelector); err == nil {
					result = append(result, matchingOwners(owners, sel)...)
				}
			}
			return result
		}

		var sources, destinations []*podTemplateOwner
		for _, rule := range policy.Spec.Ingress {
			sources = append(sources, peerOwners(rule.From)...)
		}
		for _, rule := range policy.Spec.Egress {
			destinations = append(destinations, peerOwners(rule.To)...)
		}

		types := make([]string, 0, len(policy.Spec.PolicyTypes))
		for _, t := range policy.Spec.PolicyTypes {
			types = append(types, string(t))
		}
		g.addNode(dto.KubeGraphNodeDto{
			ID: id, Kind: "NetworkPolicy", Name: policy.Name, Namespace: namespace, Status: statusHealthy, Weight: 1,
			Summary: fmt.Sprintf("%s · %d targets", strings.Join(types, "+"), len(targets)),
			Details: map[string]string{"External peers": fmt.Sprint(external)},
		})

		for _, target := range targets {
			g.addEdge(id, target.node.ID, "applies")
			for _, src := range sources {
				g.addEdge(src.node.ID, target.node.ID, "allows")
			}
			for _, dst := range destinations {
				g.addEdge(target.node.ID, dst.node.ID, "allows")
			}
		}
	}
	return nil
}

// addQuotas adds the namespace's ResourceQuotas and LimitRanges.
func (s *kubeTopologyService) addQuotas(ctx context.Context, c *kube.Cluster, namespace string, _ *namespaceOwners, g *graphBuilder) error {
	quotas, err := c.Clientset.CoreV1().ResourceQuotas(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return optionalStep(err, "list resourcequotas")
	}
	for i := range quotas.Items {
		q := &quotas.Items[i]
		status, summary := quotaStatus(q)
		g.addNode(dto.KubeGraphNodeDto{
			ID: kubeNodeID("ResourceQuota", namespace, q.Name), Kind: "ResourceQuota", Name: q.Name, Namespace: namespace,
			Status: status, Summary: summary, Weight: 1,
		})
	}

	limits, err := c.Clientset.CoreV1().LimitRanges(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return optionalStep(err, "list limitranges")
	}
	for _, lr := range limits.Items {
		g.addNode(dto.KubeGraphNodeDto{
			ID: kubeNodeID("LimitRange", namespace, lr.Name), Kind: "LimitRange", Name: lr.Name, Namespace: namespace,
			Status: statusHealthy, Summary: fmt.Sprintf("%d limits", len(lr.Spec.Limits)), Weight: 1,
		})
	}
	return nil
}

// quotaStatus reports usage against the hard limits, flagging those close to full.
func quotaStatus(q *corev1.ResourceQuota) (string, string) {
	names := make([]string, 0, len(q.Status.Hard))
	for name := range q.Status.Hard {
		names = append(names, string(name))
	}
	sort.Strings(names)

	status := statusHealthy
	parts := make([]string, 0, len(names))
	for _, name := range names {
		hard := q.Status.Hard[corev1.ResourceName(name)]
		used := q.Status.Used[corev1.ResourceName(name)]
		parts = append(parts, fmt.Sprintf("%s %s/%s", name, used.String(), hard.String()))
		if hard.IsZero() {
			continue
		}
		ratio := used.AsApproximateFloat64() / hard.AsApproximateFloat64()
		switch {
		case ratio >= 1:
			status = statusError
		case ratio >= 0.9:
			status = worstStatus(status, statusWarning)
		}
	}
	return status, strings.Join(parts, " · ")
}

func matchingOwners(owners *namespaceOwners, selector labels.Selector) []*podTemplateOwner {
	var result []*podTemplateOwner
	for _, o := range owners.owners {
		if selector.Matches(labels.Set(o.labels)) {
			result = append(result, o)
		}
	}
	return result
}

// optionalStep skips a family of objects the user can't read or the cluster doesn't serve.
func optionalStep(err error, action string) error {
	if kube.IsOptional(err) {
		return nil
	}
	return kube.WrapError(err, action)
}
