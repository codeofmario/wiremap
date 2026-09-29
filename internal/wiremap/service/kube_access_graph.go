package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/codeofmario/wiremap/internal/wiremap/dto"
	"github.com/codeofmario/wiremap/internal/wiremap/kube"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Kubernetes' own bindings (system:*) are hidden: a fresh cluster has dozens.
const systemPrefix = "system:"

// accessGraph is the cluster-wide access lens: subject → binding → role, for
// every RoleBinding and ClusterRoleBinding that isn't a built-in system one.
func (s *kubeTopologyService) accessGraph(ctx context.Context, c *kube.Cluster) (*dto.KubeGraphDto, error) {
	clusterBindings, err := c.Clientset.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, kube.WrapError(err, "list clusterrolebindings")
	}
	roleBindings, err := c.Clientset.RbacV1().RoleBindings("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, kube.WrapError(err, "list rolebindings")
	}
	rules := s.roleRules(ctx, c, "")

	g := newGraph(LevelCluster)
	for _, b := range clusterBindings.Items {
		if isSystemBinding(b.Name, b.Subjects) {
			continue
		}
		addAccessBinding(g, "ClusterRoleBinding", "", b.Name, b.Subjects, b.RoleRef, rules)
	}
	for _, b := range roleBindings.Items {
		if isSystemBinding(b.Name, b.Subjects) {
			continue
		}
		addAccessBinding(g, "RoleBinding", b.Namespace, b.Name, b.Subjects, b.RoleRef, rules)
	}
	return g.result(), nil
}

func addAccessBinding(g *graphBuilder, kind, namespace, name string, subjects []rbacv1.Subject, roleRef rbacv1.RoleRef, rules map[string]int) {
	bindingID := kubeNodeID(kind, namespace, name)
	summary := roleRef.Kind + " " + roleRef.Name
	if namespace != "" {
		summary = namespace + " · " + summary
	}
	g.addNode(dto.KubeGraphNodeDto{ID: bindingID, Kind: kind, Name: name, Namespace: namespace, Status: statusHealthy, Summary: summary, Weight: 1})

	for _, sub := range subjects {
		subNamespace := ""
		if sub.Kind == rbacv1.ServiceAccountKind {
			subNamespace = sub.Namespace
			if subNamespace == "" {
				subNamespace = namespace
			}
		}
		subID := kubeNodeID(sub.Kind, subNamespace, sub.Name)
		subSummary := strings.ToLower(sub.Kind)
		if subNamespace != "" {
			subSummary = subNamespace
		}
		g.addNode(dto.KubeGraphNodeDto{ID: subID, Kind: sub.Kind, Name: sub.Name, Namespace: subNamespace, Status: statusHealthy, Summary: subSummary, Weight: 1})
		g.addEdge(bindingID, subID, "binds")
	}

	roleNamespace := namespace
	if roleRef.Kind == "ClusterRole" {
		roleNamespace = ""
	}
	roleID := kubeNodeID(roleRef.Kind, roleNamespace, roleRef.Name)
	roleSummary := ""
	if count, ok := rules[roleID]; ok {
		roleSummary = fmt.Sprintf("%d rules", count)
	}
	g.addNode(dto.KubeGraphNodeDto{ID: roleID, Kind: roleRef.Kind, Name: roleRef.Name, Namespace: roleNamespace, Status: statusHealthy, Summary: roleSummary, Weight: 1})
	g.addEdge(bindingID, roleID, "grants")
}

func isSystemBinding(name string, subjects []rbacv1.Subject) bool {
	if strings.HasPrefix(name, systemPrefix) {
		return true
	}
	for _, sub := range subjects {
		if !strings.HasPrefix(sub.Name, systemPrefix) && !strings.HasPrefix(sub.Namespace, "kube-") {
			return false
		}
	}
	return len(subjects) > 0
}
