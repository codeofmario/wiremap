package service

import (
	"context"
	"fmt"

	"github.com/codeofmario/wiremap/internal/wiremap/dto"
	"github.com/codeofmario/wiremap/internal/wiremap/kube"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const defaultServiceAccount = "default"

// addServiceAccounts draws workload → ServiceAccount → (Cluster)RoleBinding → (Cluster)Role
// for the accounts workloads run as. The "default" account is only drawn when
// something binds permissions to it, since every namespace has one.
func (s *kubeTopologyService) addServiceAccounts(ctx context.Context, c *kube.Cluster, namespace string, owners *namespaceOwners, g *graphBuilder) error {
	usedBy := map[string][]string{}
	for _, o := range owners.owners {
		account := o.spec.ServiceAccountName
		if account == "" {
			account = defaultServiceAccount
		}
		usedBy[account] = append(usedBy[account], o.node.ID)
	}
	if len(usedBy) == 0 {
		return nil
	}

	bindings, err := s.bindingsFor(ctx, c, namespace, usedBy)
	if err != nil {
		return err
	}

	addAccount := func(name string) string {
		id := kubeNodeID("ServiceAccount", namespace, name)
		if !g.hasNode(id) {
			g.addNode(dto.KubeGraphNodeDto{
				ID: id, Kind: "ServiceAccount", Name: name, Namespace: namespace, Status: statusHealthy, Weight: 1,
				Summary: fmt.Sprintf("used by %d", len(usedBy[name])),
			})
			for _, owner := range usedBy[name] {
				g.addEdge(owner, id, "runs-as")
			}
		}
		return id
	}

	for name := range usedBy {
		if name != defaultServiceAccount {
			addAccount(name)
		}
	}

	roles := s.roleRules(ctx, c, namespace)
	for _, b := range bindings {
		bindingID := kubeNodeID(b.kind, b.namespace, b.name)
		g.addNode(dto.KubeGraphNodeDto{
			ID: bindingID, Kind: b.kind, Name: b.name, Namespace: b.namespace, Status: statusHealthy, Weight: 1,
			Summary: b.roleRef.Kind + " " + b.roleRef.Name,
		})
		for _, account := range b.accounts {
			g.addEdge(bindingID, addAccount(account), "binds")
		}

		roleNamespace := namespace
		if b.roleRef.Kind == "ClusterRole" {
			roleNamespace = ""
		}
		roleID := kubeNodeID(b.roleRef.Kind, roleNamespace, b.roleRef.Name)
		summary := ""
		if count, ok := roles[roleID]; ok {
			summary = fmt.Sprintf("%d rules", count)
		}
		g.addNode(dto.KubeGraphNodeDto{
			ID: roleID, Kind: b.roleRef.Kind, Name: b.roleRef.Name, Namespace: roleNamespace,
			Status: statusHealthy, Summary: summary, Weight: 1,
		})
		g.addEdge(bindingID, roleID, "grants")
	}
	return nil
}

type accountBinding struct {
	kind      string
	name      string
	namespace string
	roleRef   rbacv1.RoleRef
	accounts  []string
}

// bindingsFor returns the RoleBindings and ClusterRoleBindings granting
// permissions to the given ServiceAccounts of the namespace.
func (s *kubeTopologyService) bindingsFor(ctx context.Context, c *kube.Cluster, namespace string, accounts map[string][]string) ([]accountBinding, error) {
	var result []accountBinding
	matchAccounts := func(subjects []rbacv1.Subject, bindingNamespace string) []string {
		var names []string
		for _, sub := range subjects {
			subNamespace := sub.Namespace
			if subNamespace == "" {
				subNamespace = bindingNamespace
			}
			if sub.Kind == rbacv1.ServiceAccountKind && subNamespace == namespace {
				if _, ok := accounts[sub.Name]; ok {
					names = append(names, sub.Name)
				}
			}
		}
		return names
	}

	roleBindings, err := c.Clientset.RbacV1().RoleBindings(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, optionalStep(err, "list rolebindings")
	}
	for _, rb := range roleBindings.Items {
		if names := matchAccounts(rb.Subjects, namespace); len(names) > 0 {
			result = append(result, accountBinding{kind: "RoleBinding", name: rb.Name, namespace: namespace, roleRef: rb.RoleRef, accounts: names})
		}
	}

	clusterBindings, err := c.Clientset.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
	if err != nil {
		return result, optionalStep(err, "list clusterrolebindings")
	}
	for _, crb := range clusterBindings.Items {
		if names := matchAccounts(crb.Subjects, ""); len(names) > 0 {
			result = append(result, accountBinding{kind: "ClusterRoleBinding", name: crb.Name, roleRef: crb.RoleRef, accounts: names})
		}
	}
	return result, nil
}

// roleRules counts the rules of the namespace's Roles (all namespaces for "") and
// all ClusterRoles, keyed by graph node ID.
func (s *kubeTopologyService) roleRules(ctx context.Context, c *kube.Cluster, namespace string) map[string]int {
	counts := map[string]int{}
	if roles, err := c.Clientset.RbacV1().Roles(namespace).List(ctx, metav1.ListOptions{}); err == nil {
		for _, r := range roles.Items {
			counts[kubeNodeID("Role", r.Namespace, r.Name)] = len(r.Rules)
		}
	}
	if clusterRoles, err := c.Clientset.RbacV1().ClusterRoles().List(ctx, metav1.ListOptions{}); err == nil {
		for _, r := range clusterRoles.Items {
			counts[kubeNodeID("ClusterRole", "", r.Name)] = len(r.Rules)
		}
	}
	return counts
}
