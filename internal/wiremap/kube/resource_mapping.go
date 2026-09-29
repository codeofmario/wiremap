package kube

import (
	"fmt"

	apperrors "github.com/codeofmario/wiremap/internal/wiremap/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ResourceMapping identifies how to reach a kind through the dynamic client.
type ResourceMapping struct {
	GVR        schema.GroupVersionResource
	Namespaced bool
}

// ResourceFor resolves a kind (and its API group, "" for core) to the resource
// the cluster serves it under. The discovery cache is refreshed once on a miss,
// so CRDs installed after startup are found too.
func (c *Cluster) ResourceFor(group, kind string) (*ResourceMapping, error) {
	gk := schema.GroupKind{Group: group, Kind: kind}
	mapping, err := c.Mapper.RESTMapping(gk)
	if meta.IsNoMatchError(err) {
		if resettable, ok := c.Mapper.(meta.ResettableRESTMapper); ok {
			resettable.Reset()
			mapping, err = c.Mapper.RESTMapping(gk)
		}
	}
	if err != nil {
		if meta.IsNoMatchError(err) {
			return nil, apperrors.NotFound(fmt.Sprintf("kind %s is not served by this cluster", gk.String()))
		}
		return nil, apperrors.Internal(fmt.Sprintf("failed to resolve kind %s: %s", gk.String(), err))
	}

	return &ResourceMapping{
		GVR:        mapping.Resource,
		Namespaced: mapping.Scope.Name() == meta.RESTScopeNameNamespace,
	}, nil
}

// Serves reports whether the cluster knows the kind, e.g. whether a CRD is installed.
func (c *Cluster) Serves(group, kind string) bool {
	_, err := c.ResourceFor(group, kind)
	return err == nil
}
