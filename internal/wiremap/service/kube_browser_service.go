package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/codeofmario/wiremap/internal/wiremap/dto"
	apperrors "github.com/codeofmario/wiremap/internal/wiremap/errors"
	"github.com/codeofmario/wiremap/internal/wiremap/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
)

const browserListLimit = 500

// KubeBrowserService lists any kind the cluster serves, including custom resources.
type KubeBrowserService interface {
	Kinds(ctx context.Context, cluster string) ([]dto.KubeKindDto, error)
	// List returns one page of objects; pass the previous page's Continue token for the next.
	List(ctx context.Context, cluster string, group string, version string, resource string, namespace string, continueToken string) (*dto.KubeObjectListDto, error)
	// Counts returns how many objects of each "group/Kind" exist in the namespace ("" for all);
	// cluster-scoped kinds are counted cluster-wide and kinds the cluster doesn't serve are left out.
	Counts(ctx context.Context, cluster string, namespace string, kinds []string) (map[string]int, error)
}

type kubeBrowserService struct {
	pool *kube.ClusterPool
}

func NewKubeBrowserService(pool *kube.ClusterPool) KubeBrowserService {
	return &kubeBrowserService{pool: pool}
}

func (s *kubeBrowserService) Kinds(ctx context.Context, clusterName string) ([]dto.KubeKindDto, error) {
	c, err := s.pool.Get(clusterName)
	if err != nil {
		return nil, err
	}

	lists, err := discovery.ServerPreferredResources(c.Clientset.Discovery())
	// A broken aggregated API (e.g. a dead metrics-server) fails only its own group
	if err != nil && !discovery.IsGroupDiscoveryFailedError(err) {
		return nil, apperrors.Internal(fmt.Sprintf("failed to discover resources: %s", err))
	}

	kinds := []dto.KubeKindDto{}
	for _, list := range lists {
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			continue
		}
		for _, r := range list.APIResources {
			if strings.Contains(r.Name, "/") || !hasVerb(r.Verbs, "list") {
				continue // subresources such as pods/log
			}
			kinds = append(kinds, dto.KubeKindDto{
				Group:      gv.Group,
				Version:    gv.Version,
				Resource:   r.Name,
				Kind:       r.Kind,
				Namespaced: r.Namespaced,
				ShortNames: r.ShortNames,
			})
		}
	}

	sort.Slice(kinds, func(i, j int) bool {
		if kinds[i].Group != kinds[j].Group {
			return kinds[i].Group < kinds[j].Group
		}
		return kinds[i].Kind < kinds[j].Kind
	})
	return kinds, nil
}

func (s *kubeBrowserService) List(ctx context.Context, clusterName string, group string, version string, resource string, namespace string, continueToken string) (*dto.KubeObjectListDto, error) {
	c, err := s.pool.Get(clusterName)
	if err != nil {
		return nil, err
	}
	if version == "" || resource == "" {
		return nil, apperrors.BadRequest("version and resource are required")
	}

	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: resource}
	list, err := c.Dynamic.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{Limit: browserListLimit, Continue: continueToken})
	if err != nil {
		return nil, kube.WrapError(err, "list "+gvr.GroupResource().String())
	}

	// The API server already returns objects sorted by namespace and name
	result := &dto.KubeObjectListDto{
		Items:    make([]dto.KubeObjectDto, 0, len(list.Items)),
		Continue: list.GetContinue(),
	}
	for i := range list.Items {
		result.Items = append(result.Items, objectDto(&list.Items[i], group))
	}
	return result, nil
}

func objectDto(u *unstructured.Unstructured, group string) dto.KubeObjectDto {
	status, summary := objectStatus(u)
	return dto.KubeObjectDto{
		Name:      u.GetName(),
		Namespace: u.GetNamespace(),
		Kind:      u.GetKind(),
		Group:     group,
		Status:    status,
		Summary:   summary,
		Created:   u.GetCreationTimestamp().Format(time.RFC3339),
		Drillable: drillableKinds[group+"/"+u.GetKind()],
	}
}

// Kinds the object topology level can open, by "group/Kind"
var drillableKinds = map[string]bool{
	"/Namespace": true, "/Node": true, "/Pod": true, "/Service": true,
	"/ConfigMap": true, "/Secret": true, "/PersistentVolumeClaim": true,
	"apps/Deployment": true, "apps/StatefulSet": true, "apps/DaemonSet": true, "apps/ReplicaSet": true,
	"batch/Job": true, "batch/CronJob": true, "networking.k8s.io/Ingress": true,
}

func (s *kubeBrowserService) Counts(ctx context.Context, clusterName string, namespace string, kinds []string) (map[string]int, error) {
	c, err := s.pool.Get(clusterName)
	if err != nil {
		return nil, err
	}

	counts := make(map[string]int, len(kinds))
	for _, key := range kinds {
		group, kind, ok := strings.Cut(key, "/")
		if !ok {
			return nil, apperrors.BadRequest(fmt.Sprintf("kind %q must be group/Kind", key))
		}
		mapping, err := c.ResourceFor(group, kind)
		if err != nil {
			continue
		}
		scope := ""
		if mapping.Namespaced {
			scope = namespace
		}
		count, err := countObjects(ctx, c, mapping.GVR, scope)
		if err != nil {
			continue // e.g. RBAC forbids listing this kind
		}
		counts[key] = count
	}
	return counts, nil
}

// countObjects reads a single-item page and uses the server's remaining-item
// count, falling back to a full list when the server doesn't report one.
func countObjects(ctx context.Context, c *kube.Cluster, gvr schema.GroupVersionResource, namespace string) (int, error) {
	resource := c.Dynamic.Resource(gvr).Namespace(namespace)
	page, err := resource.List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return 0, err
	}
	if page.GetContinue() == "" {
		return len(page.Items), nil
	}
	if remaining := page.GetRemainingItemCount(); remaining != nil {
		return len(page.Items) + int(*remaining), nil
	}
	all, err := resource.List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, err
	}
	return len(all.Items), nil
}

func hasVerb(verbs metav1.Verbs, verb string) bool {
	for _, v := range verbs {
		if v == verb {
			return true
		}
	}
	return false
}
