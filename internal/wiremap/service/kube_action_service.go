package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/codeofmario/wiremap/internal/wiremap/dto"
	apperrors "github.com/codeofmario/wiremap/internal/wiremap/errors"
	"github.com/codeofmario/wiremap/internal/wiremap/kube"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/yaml"
)

// ObjectRef identifies any object in a cluster; Group is "" for core kinds.
type ObjectRef struct {
	Cluster   string
	Group     string
	Kind      string
	Namespace string
	Name      string
}

// KubeActionService changes cluster state. Every action goes through the
// Kubernetes API as the configured identity, so cluster RBAC decides what's allowed.
type KubeActionService interface {
	Update(ctx context.Context, ref ObjectRef, manifest string) error
	Delete(ctx context.Context, ref ObjectRef) error
	Scale(ctx context.Context, ref ObjectRef, replicas int32) error
	Restart(ctx context.Context, ref ObjectRef) error
	TriggerCronJob(ctx context.Context, ref ObjectRef) (string, error)
	SetSuspended(ctx context.Context, ref ObjectRef, suspended bool) error
}

type kubeActionService struct {
	pool *kube.ClusterPool
}

func NewKubeActionService(pool *kube.ClusterPool) KubeActionService {
	return &kubeActionService{pool: pool}
}

// Kinds whose pods can be restarted by bumping a pod template annotation, like `kubectl rollout restart`
var restartableKinds = map[string]bool{"apps/Deployment": true, "apps/StatefulSet": true, "apps/DaemonSet": true}

// resourceClient resolves the dynamic client for the object's kind and namespace.
func (s *kubeActionService) resourceClient(ref ObjectRef) (*kube.Cluster, func() dynamic.ResourceInterface, error) {
	c, err := s.pool.Get(ref.Cluster)
	if err != nil {
		return nil, nil, err
	}
	mapping, err := c.ResourceFor(ref.Group, ref.Kind)
	if err != nil {
		return nil, nil, err
	}
	client := func() dynamic.ResourceInterface {
		if mapping.Namespaced {
			return c.Dynamic.Resource(mapping.GVR).Namespace(ref.Namespace)
		}
		return c.Dynamic.Resource(mapping.GVR)
	}
	return c, client, nil
}

// Update replaces an object with an edited manifest. The manifest keeps its
// resourceVersion, so edits made in the meantime are reported as a conflict.
func (s *kubeActionService) Update(ctx context.Context, ref ObjectRef, manifest string) error {
	_, client, err := s.resourceClient(ref)
	if err != nil {
		return err
	}

	edited := &unstructured.Unstructured{}
	if err := yaml.Unmarshal([]byte(manifest), &edited.Object); err != nil {
		return apperrors.BadRequest(fmt.Sprintf("invalid yaml: %s", err))
	}
	if edited.GetKind() != ref.Kind || edited.GetName() != ref.Name || edited.GetNamespace() != ref.Namespace {
		return apperrors.BadRequest("the manifest's kind, name and namespace can't be changed")
	}

	if ref.Group == "" && ref.Kind == "Secret" {
		current, err := client().Get(ctx, ref.Name, metav1.GetOptions{})
		if err != nil {
			return kube.WrapError(err, "get secret")
		}
		restoreRedactedSecret(edited, current)
	}

	if _, err := client().Update(ctx, edited, metav1.UpdateOptions{}); err != nil {
		return kube.WrapError(err, fmt.Sprintf("update %s %s", ref.Kind, ref.Name))
	}
	return nil
}

// restoreRedactedSecret merges an edited Secret with the stored one: keys still
// showing the redaction placeholder keep their stored value, removed keys are
// deleted, and new values are written through stringData.
func restoreRedactedSecret(edited, current *unstructured.Unstructured) {
	stored, _, _ := unstructured.NestedStringMap(current.Object, "data")
	editedValues, _, _ := unstructured.NestedStringMap(edited.Object, "stringData")

	data := map[string]interface{}{}
	stringData := map[string]interface{}{}
	for key, value := range editedValues {
		if value == redactedValue {
			if old, ok := stored[key]; ok {
				data[key] = old
			}
			continue
		}
		stringData[key] = value
	}

	edited.Object["data"] = data
	if len(stringData) > 0 {
		edited.Object["stringData"] = stringData
	} else {
		delete(edited.Object, "stringData")
	}
}

func (s *kubeActionService) Delete(ctx context.Context, ref ObjectRef) error {
	_, client, err := s.resourceClient(ref)
	if err != nil {
		return err
	}
	propagation := metav1.DeletePropagationBackground
	if err := client().Delete(ctx, ref.Name, metav1.DeleteOptions{PropagationPolicy: &propagation}); err != nil {
		return kube.WrapError(err, fmt.Sprintf("delete %s %s", ref.Kind, ref.Name))
	}
	return nil
}

// Scale goes through the scale subresource, so it works for any kind that
// supports it, custom resources included.
func (s *kubeActionService) Scale(ctx context.Context, ref ObjectRef, replicas int32) error {
	if replicas < 0 {
		return apperrors.BadRequest("replicas can't be negative")
	}
	_, client, err := s.resourceClient(ref)
	if err != nil {
		return err
	}
	patch := fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas)
	if _, err := client().Patch(ctx, ref.Name, types.MergePatchType, []byte(patch), metav1.PatchOptions{}, "scale"); err != nil {
		return kube.WrapError(err, fmt.Sprintf("scale %s %s", ref.Kind, ref.Name))
	}
	return nil
}

func (s *kubeActionService) Restart(ctx context.Context, ref ObjectRef) error {
	if !restartableKinds[ref.Group+"/"+ref.Kind] {
		return apperrors.BadRequest(fmt.Sprintf("%s can't be restarted", ref.Kind))
	}
	_, client, err := s.resourceClient(ref)
	if err != nil {
		return err
	}
	patch, _ := json.Marshal(map[string]interface{}{
		"spec": map[string]interface{}{"template": map[string]interface{}{"metadata": map[string]interface{}{
			"annotations": map[string]string{"kubectl.kubernetes.io/restartedAt": time.Now().Format(time.RFC3339)},
		}}},
	})
	if _, err := client().Patch(ctx, ref.Name, types.MergePatchType, patch, metav1.PatchOptions{}); err != nil {
		return kube.WrapError(err, fmt.Sprintf("restart %s %s", ref.Kind, ref.Name))
	}
	return nil
}

// TriggerCronJob starts a Job from the CronJob's template now, like `kubectl create job --from=cronjob/...`.
func (s *kubeActionService) TriggerCronJob(ctx context.Context, ref ObjectRef) (string, error) {
	c, err := s.pool.Get(ref.Cluster)
	if err != nil {
		return "", err
	}
	cj, err := c.Clientset.BatchV1().CronJobs(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return "", kube.WrapError(err, "get cronjob")
	}

	controller := true
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:        truncateName(fmt.Sprintf("%s-manual-%d", cj.Name, time.Now().Unix())),
			Namespace:   ref.Namespace,
			Labels:      cj.Spec.JobTemplate.Labels,
			Annotations: map[string]string{"cronjob.kubernetes.io/instantiate": "manual"},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "batch/v1", Kind: "CronJob", Name: cj.Name, UID: cj.UID, Controller: &controller,
			}},
		},
		Spec: cj.Spec.JobTemplate.Spec,
	}
	created, err := c.Clientset.BatchV1().Jobs(ref.Namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return "", kube.WrapError(err, "create job")
	}
	return created.Name, nil
}

func (s *kubeActionService) SetSuspended(ctx context.Context, ref ObjectRef, suspended bool) error {
	if ref.Group != "batch" || ref.Kind != "CronJob" {
		return apperrors.BadRequest(fmt.Sprintf("%s can't be suspended", ref.Kind))
	}
	_, client, err := s.resourceClient(ref)
	if err != nil {
		return err
	}
	patch := fmt.Sprintf(`{"spec":{"suspend":%t}}`, suspended)
	if _, err := client().Patch(ctx, ref.Name, types.MergePatchType, []byte(patch), metav1.PatchOptions{}); err != nil {
		return kube.WrapError(err, fmt.Sprintf("suspend %s %s", ref.Kind, ref.Name))
	}
	return nil
}

// Object names are limited to 63 characters when used as labels (e.g. job-name).
func truncateName(name string) string {
	if len(name) > 63 {
		return name[:63]
	}
	return name
}

// actionsFor lists what can be done to an object from the UI, with the state
// the controls need (current replicas, suspended, cordoned).
func actionsFor(obj *unstructured.Unstructured, group, kind string) dto.KubeActionsDto {
	actions := dto.KubeActionsDto{Edit: true, Delete: true}
	if replicas, found, _ := unstructured.NestedInt64(obj.Object, "spec", "replicas"); found {
		r := int32(replicas)
		actions.Replicas = &r
	}
	actions.Restart = restartableKinds[group+"/"+kind]
	switch group + "/" + kind {
	case "batch/CronJob":
		suspended, _, _ := unstructured.NestedBool(obj.Object, "spec", "suspend")
		actions.Trigger = true
		actions.Suspended = &suspended
	case "/Node":
		unschedulable, _, _ := unstructured.NestedBool(obj.Object, "spec", "unschedulable")
		actions.Unschedulable = &unschedulable
	}
	return actions
}
