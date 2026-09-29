package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/codeofmario/wiremap/internal/wiremap/dto"
	apperrors "github.com/codeofmario/wiremap/internal/wiremap/errors"
	"github.com/codeofmario/wiremap/internal/wiremap/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"sigs.k8s.io/yaml"
)

const redactedValue = "<redacted>"

type KubeResourceService interface {
	Get(ctx context.Context, cluster string, group string, kind string, namespace string, name string) (*dto.KubeResourceDto, error)
}

type kubeResourceService struct {
	pool *kube.ClusterPool
}

func NewKubeResourceService(pool *kube.ClusterPool) KubeResourceService {
	return &kubeResourceService{pool: pool}
}

func (s *kubeResourceService) Get(ctx context.Context, clusterName string, group string, kind string, namespace string, name string) (*dto.KubeResourceDto, error) {
	c, err := s.pool.Get(clusterName)
	if err != nil {
		return nil, err
	}

	obj, err := getObject(ctx, c, group, kind, namespace, name)
	if err != nil {
		return nil, err
	}
	actions := actionsFor(obj, group, kind)

	manifest, err := toYAML(obj)
	if err != nil {
		return nil, err
	}

	events, err := s.events(ctx, c, kind, namespace, name)
	if err != nil {
		return nil, err
	}

	return &dto.KubeResourceDto{
		Kind:      kind,
		Group:     group,
		Name:      name,
		Namespace: namespace,
		YAML:      manifest,
		Events:    events,
		Actions:   actions,
	}, nil
}

// getObject fetches any kind the cluster serves, with Secret values redacted.
func getObject(ctx context.Context, c *kube.Cluster, group, kind, namespace, name string) (*unstructured.Unstructured, error) {
	mapping, err := c.ResourceFor(group, kind)
	if err != nil {
		return nil, err
	}

	client := c.Dynamic.Resource(mapping.GVR)
	var obj *unstructured.Unstructured
	if mapping.Namespaced {
		obj, err = client.Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	} else {
		obj, err = client.Get(ctx, name, metav1.GetOptions{})
	}
	if err != nil {
		return nil, kube.WrapError(err, fmt.Sprintf("get %s %s", kind, name))
	}

	if group == "" && kind == "Secret" {
		redactSecret(obj)
	}
	return obj, nil
}

// redactSecret keeps the keys of a Secret but never sends its values to the browser.
func redactSecret(secret *unstructured.Unstructured) {
	redacted := map[string]interface{}{}
	for _, field := range []string{"data", "stringData"} {
		values, _, _ := unstructured.NestedMap(secret.Object, field)
		for key := range values {
			redacted[key] = redactedValue
		}
		unstructured.RemoveNestedField(secret.Object, field)
	}
	if len(redacted) > 0 {
		secret.Object["stringData"] = redacted
	}
	unstructured.RemoveNestedField(secret.Object, "metadata", "annotations", "kubectl.kubernetes.io/last-applied-configuration")
}

func toYAML(obj *unstructured.Unstructured) (string, error) {
	unstructured.RemoveNestedField(obj.Object, "metadata", "managedFields")

	out, err := yaml.Marshal(obj.Object)
	if err != nil {
		return "", apperrors.Internal(fmt.Sprintf("failed to encode yaml: %s", err))
	}
	return string(out), nil
}

func (s *kubeResourceService) events(ctx context.Context, c *kube.Cluster, kind, namespace, name string) ([]dto.KubeEventDto, error) {
	selector := fields.Set{"involvedObject.kind": kind, "involvedObject.name": name}.AsSelector().String()
	list, err := c.Clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{FieldSelector: selector})
	if err != nil {
		if kube.IsOptional(err) {
			return []dto.KubeEventDto{}, nil
		}
		return nil, kube.WrapError(err, "list events")
	}

	sort.Slice(list.Items, func(i, j int) bool {
		return eventTime(&list.Items[i]).After(eventTime(&list.Items[j]))
	})

	result := make([]dto.KubeEventDto, 0, len(list.Items))
	for i := range list.Items {
		e := &list.Items[i]
		result = append(result, dto.KubeEventDto{
			Type:     e.Type,
			Reason:   e.Reason,
			Message:  e.Message,
			Count:    e.Count,
			LastSeen: eventTime(e).Format(time.RFC3339),
			Object:   e.InvolvedObject.Kind + "/" + e.InvolvedObject.Name,
		})
	}
	return result, nil
}

func eventTime(e *corev1.Event) time.Time {
	switch {
	case !e.LastTimestamp.IsZero():
		return e.LastTimestamp.Time
	case !e.EventTime.IsZero():
		return e.EventTime.Time
	default:
		return e.CreationTimestamp.Time
	}
}
