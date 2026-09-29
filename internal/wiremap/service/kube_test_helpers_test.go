package service

import (
	"github.com/codeofmario/wiremap/internal/wiremap/kube"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/meta/testrestmapper"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
)

// Argo Rollout stands in for any CRD-defined controller in tests
var (
	rolloutGVK = schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Rollout"}
	rolloutGVR = schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "rollouts"}
)

// newTestPool builds a cluster whose typed and dynamic clients share the given
// objects; unstructured objects (custom resources) only exist in the dynamic one.
func newTestPool(objects ...runtime.Object) *kube.ClusterPool {
	pool, _ := newTestCluster(objects...)
	return pool
}

func newTestCluster(objects ...runtime.Object) (*kube.ClusterPool, *fake.Clientset) {
	var typed []runtime.Object
	for _, o := range objects {
		if _, ok := o.(*unstructured.Unstructured); !ok {
			typed = append(typed, o)
		}
	}

	custom := meta.NewDefaultRESTMapper([]schema.GroupVersion{rolloutGVK.GroupVersion()})
	custom.Add(rolloutGVK, meta.RESTScopeNamespace)
	mapper := meta.MultiRESTMapper{testrestmapper.TestOnlyStaticRESTMapper(scheme.Scheme), custom}

	clientset := fake.NewSimpleClientset(typed...)
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme.Scheme,
		map[schema.GroupVersionResource]string{rolloutGVR: "RolloutList"}, objects...)

	return kube.NewStaticClusterPool(&kube.Cluster{
		Name:      "test",
		Clientset: clientset,
		Dynamic:   dynamicClient,
		Mapper:    mapper,
	}), clientset
}

func rollout(name, uid string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{Object: map[string]interface{}{
		"status": map[string]interface{}{
			"conditions": []interface{}{map[string]interface{}{"type": "Available", "status": "True"}},
		},
	}}
	u.SetGroupVersionKind(rolloutGVK)
	u.SetNamespace(testNS)
	u.SetName(name)
	u.SetUID(types.UID(uid))
	return u
}

func runtimeToUnstructured(obj runtime.Object) (map[string]interface{}, error) {
	return runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
}
