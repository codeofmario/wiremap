package service

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

func TestPodGraphShowsOnlyContainers(t *testing.T) {
	pod := runningPod(objectMeta("web-a", "p1"), "node-a", nil)
	pod.Spec.InitContainers = []corev1.Container{{Name: "migrate", Image: "migrate:1"}}
	pod.Status.InitContainerStatuses = []corev1.ContainerStatus{{
		Name: "migrate", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "Completed"}},
	}}
	pod.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{{Name: "config", MountPath: "/etc/app"}}
	pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{Name: "sidecar", Image: "proxy:1"})
	pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, corev1.ContainerStatus{
		Name: "sidecar", State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
	})
	pod.Spec.Volumes = []corev1.Volume{{Name: "config", VolumeSource: corev1.VolumeSource{
		ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "web-config"}},
	}}}
	pod.Status.ContainerStatuses[0].State = corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}

	g, err := NewKubeTopologyService(newTestPool(pod)).Object(context.Background(), "test", testNS, "", "Pod", "web-a")
	if err != nil {
		t.Fatal(err)
	}

	nodes := nodeIDs(g)
	podID := kubeNodeID("Pod", testNS, "web-a")
	app := kubeNodeID("Container", testNS, "web-a/app")
	sidecar := kubeNodeID("Container", testNS, "web-a/sidecar")
	migrate := kubeNodeID("Container", testNS, "web-a/migrate")

	if nodes[podID].Drillable {
		t.Error("the pod being viewed should not be drillable")
	}
	for _, id := range []string{app, sidecar, migrate} {
		if !hasEdge(g, podID, id, "owns") || !nodes[id].Drillable {
			t.Errorf("missing drillable container %s", id)
		}
	}
	if len(g.Nodes) != 4 {
		t.Errorf("pod view should hold only the pod and its containers: %+v", g.Nodes)
	}
	if s := nodes[app].Status; s != statusHealthy {
		t.Errorf("running ready container status %q", s)
	}
	if s := nodes[sidecar].Status; s != statusError {
		t.Errorf("crash-looping container status %q", s)
	}
	if s := nodes[migrate].Status; s != statusIdle {
		t.Errorf("completed init container status %q", s)
	}
}

func TestServiceGraphShowsSelectedPods(t *testing.T) {
	pool := newTestPool(
		&corev1.Service{
			ObjectMeta: objectMeta("web", "svc-web"),
			Spec:       corev1.ServiceSpec{Selector: map[string]string{"app": "web"}},
		},
		runningPod(objectMeta("web-a", "p1"), "node-a", map[string]string{"app": "web"}),
		runningPod(objectMeta("db-a", "p2"), "node-a", map[string]string{"app": "db"}),
	)

	g, err := NewKubeTopologyService(pool).Object(context.Background(), "test", testNS, "", "Service", "web")
	if err != nil {
		t.Fatal(err)
	}

	svc := kubeNodeID("Service", testNS, "web")
	if !hasEdge(g, svc, kubeNodeID("Pod", testNS, "web-a"), "selects") {
		t.Errorf("missing service → pod edge: %+v", g.Edges)
	}
	if _, ok := nodeIDs(g)[kubeNodeID("Pod", testNS, "db-a")]; ok {
		t.Error("pods outside the selector should be excluded")
	}
}

func TestIngressGraphShowsServicesOnly(t *testing.T) {
	pathType := networkingv1.PathTypePrefix
	pool := newTestPool(
		&networkingv1.Ingress{
			ObjectMeta: objectMeta("shop", "ing"),
			Spec: networkingv1.IngressSpec{Rules: []networkingv1.IngressRule{{
				Host: "shop.local",
				IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{{
						Path: "/", PathType: &pathType,
						Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{Name: "web"}},
					}},
				}},
			}}},
		},
		&corev1.Service{
			ObjectMeta: objectMeta("web", "svc-web"),
			Spec:       corev1.ServiceSpec{Selector: map[string]string{"app": "web"}},
		},
		runningPod(objectMeta("web-a", "p1"), "node-a", map[string]string{"app": "web"}),
	)

	g, err := NewKubeTopologyService(pool).Object(context.Background(), "test", testNS, "networking.k8s.io", "Ingress", "shop")
	if err != nil {
		t.Fatal(err)
	}

	svc := kubeNodeID("Service", testNS, "web")
	if !hasEdge(g, kubeNodeID("Ingress", testNS, "shop"), svc, "routes") || !nodeIDs(g)[svc].Drillable {
		t.Errorf("ingress should open to drillable services: %+v", g)
	}
	if _, ok := nodeIDs(g)[kubeNodeID("Pod", testNS, "web-a")]; ok {
		t.Error("service pods belong one level deeper")
	}
}

func TestConfigMapGraphShowsConsumingPods(t *testing.T) {
	consumer := runningPod(objectMeta("web-a", "p1"), "node-a", nil)
	consumer.Spec.Containers[0].EnvFrom = []corev1.EnvFromSource{{
		ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "web-config"}},
	}}
	pool := newTestPool(
		&corev1.ConfigMap{ObjectMeta: objectMeta("web-config", "cm")},
		consumer,
		runningPod(objectMeta("other", "p2"), "node-a", nil),
	)

	g, err := NewKubeTopologyService(pool).Object(context.Background(), "test", testNS, "", "ConfigMap", "web-config")
	if err != nil {
		t.Fatal(err)
	}

	if !hasEdge(g, kubeNodeID("Pod", testNS, "web-a"), kubeNodeID("ConfigMap", testNS, "web-config"), "uses") {
		t.Errorf("missing pod → configmap edge: %+v", g.Edges)
	}
	if _, ok := nodeIDs(g)[kubeNodeID("Pod", testNS, "other")]; ok {
		t.Error("pods not using the ConfigMap should be excluded")
	}
}
