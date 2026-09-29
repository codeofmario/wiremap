package service

import (
	"context"
	"fmt"

	"github.com/codeofmario/wiremap/internal/wiremap/dto"
	"github.com/codeofmario/wiremap/internal/wiremap/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Gateway API route kinds drawn between Gateways and Services
var gatewayRouteKinds = []string{"HTTPRoute", "GRPCRoute", "TLSRoute", "TCPRoute", "UDPRoute"}

// addGatewayRoutes draws Gateway → Route → Service when the Gateway API CRDs are installed.
func (s *kubeTopologyService) addGatewayRoutes(ctx context.Context, c *kube.Cluster, namespace string, _ *namespaceOwners, g *graphBuilder) error {
	if c.Dynamic == nil || !c.Serves(gatewayGroup, "Gateway") {
		return nil
	}

	gateways, err := listUnstructured(ctx, c, "Gateway", namespace)
	if err != nil {
		return optionalStep(err, "list gateways")
	}
	for i := range gateways {
		addGateway(g, &gateways[i], namespace)
	}

	for _, kind := range gatewayRouteKinds {
		if !c.Serves(gatewayGroup, kind) {
			continue
		}
		routes, err := listUnstructured(ctx, c, kind, namespace)
		if err != nil {
			return optionalStep(err, "list "+kind)
		}
		for i := range routes {
			addRoute(g, &routes[i], kind, namespace)
		}
	}
	return nil
}

func listUnstructured(ctx context.Context, c *kube.Cluster, kind, namespace string) ([]unstructured.Unstructured, error) {
	mapping, err := c.ResourceFor(gatewayGroup, kind)
	if err != nil {
		return nil, err
	}
	list, err := c.Dynamic.Resource(mapping.GVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func addGateway(g *graphBuilder, gw *unstructured.Unstructured, namespace string) {
	status, _ := genericStatus(gw)
	class, _, _ := unstructured.NestedString(gw.Object, "spec", "gatewayClassName")
	listeners, _, _ := unstructured.NestedSlice(gw.Object, "spec", "listeners")
	g.addNode(dto.KubeGraphNodeDto{
		ID: kubeNodeID("Gateway", namespace, gw.GetName()), Kind: "Gateway", Name: gw.GetName(), Namespace: namespace,
		Status: status, Summary: fmt.Sprintf("%s · %d listeners", class, len(listeners)), Weight: 1,
	})
}

// addRoute links a route to its parent Gateways (drawing ones from other
// namespaces, as shared gateways usually live elsewhere) and to its backend Services.
func addRoute(g *graphBuilder, route *unstructured.Unstructured, kind, namespace string) {
	id := kubeNodeID(kind, namespace, route.GetName())
	hostnames, _, _ := unstructured.NestedStringSlice(route.Object, "spec", "hostnames")
	summary := "*"
	if len(hostnames) > 0 {
		summary = hostnames[0]
		if len(hostnames) > 1 {
			summary += fmt.Sprintf(" +%d", len(hostnames)-1)
		}
	}
	status, _ := routeStatus(route)
	g.addNode(dto.KubeGraphNodeDto{
		ID: id, Kind: kind, Name: route.GetName(), Namespace: namespace, Status: status, Summary: summary, Weight: 1,
	})

	parents, _, _ := unstructured.NestedSlice(route.Object, "spec", "parentRefs")
	for _, raw := range parents {
		ref, _ := raw.(map[string]interface{})
		if refKind, _ := ref["kind"].(string); refKind != "" && refKind != "Gateway" {
			continue
		}
		name, _ := ref["name"].(string)
		gwNamespace, _ := ref["namespace"].(string)
		if gwNamespace == "" {
			gwNamespace = namespace
		}
		gwID := kubeNodeID("Gateway", gwNamespace, name)
		if !g.hasNode(gwID) {
			g.addNode(dto.KubeGraphNodeDto{
				ID: gwID, Kind: "Gateway", Name: name, Namespace: gwNamespace, Status: statusIdle,
				Summary: "in namespace " + gwNamespace, Weight: 1,
			})
		}
		g.addEdge(gwID, id, "routes")
	}

	rules, _, _ := unstructured.NestedSlice(route.Object, "spec", "rules")
	for _, rawRule := range rules {
		rule, _ := rawRule.(map[string]interface{})
		backends, _, _ := unstructured.NestedSlice(rule, "backendRefs")
		for _, rawBackend := range backends {
			backend, _ := rawBackend.(map[string]interface{})
			if backendKind, _ := backend["kind"].(string); backendKind != "" && backendKind != "Service" {
				continue
			}
			name, _ := backend["name"].(string)
			if target := kubeNodeID("Service", namespace, name); g.hasNode(target) {
				g.addEdge(id, target, "routes")
			}
		}
	}
}

// routeStatus reads the Accepted condition each parent Gateway reports for the route.
func routeStatus(route *unstructured.Unstructured) (string, string) {
	parents, _, _ := unstructured.NestedSlice(route.Object, "status", "parents")
	if len(parents) == 0 {
		return statusWarning, "not attached"
	}
	for _, raw := range parents {
		parent, _ := raw.(map[string]interface{})
		conditions, _, _ := unstructured.NestedSlice(parent, "conditions")
		for _, rawCond := range conditions {
			cond, _ := rawCond.(map[string]interface{})
			if cond["type"] == "Accepted" && cond["status"] != "True" {
				return statusError, "not accepted"
			}
		}
	}
	return statusHealthy, "accepted"
}
