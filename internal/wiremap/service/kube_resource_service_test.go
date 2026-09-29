package service

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestResourceGetRedactsSecretValues(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: objectMeta("db", "sec-db"),
		Data:       map[string][]byte{"password": []byte("hunter2")},
	}
	secret.Annotations = map[string]string{"kubectl.kubernetes.io/last-applied-configuration": `{"data":{"password":"aHVudGVyMg=="}}`}

	res, err := NewKubeResourceService(newTestPool(secret)).Get(context.Background(), "test", "", "Secret", testNS, "db")
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(res.YAML, "hunter2") || strings.Contains(res.YAML, "aHVudGVyMg") {
		t.Fatalf("secret value leaked:\n%s", res.YAML)
	}
	if !strings.Contains(res.YAML, "password: "+redactedValue) {
		t.Errorf("secret key should be listed as redacted:\n%s", res.YAML)
	}
	if !strings.Contains(res.YAML, "kind: Secret") || !strings.Contains(res.YAML, "apiVersion: v1") {
		t.Errorf("type meta missing:\n%s", res.YAML)
	}
}

func TestResourceGetReturnsEvents(t *testing.T) {
	pool := newTestPool(
		&corev1.ConfigMap{ObjectMeta: objectMeta("web-config", "cm")},
		&corev1.Event{
			ObjectMeta:     objectMeta("web-config.1", "ev"),
			InvolvedObject: corev1.ObjectReference{Kind: "ConfigMap", Name: "web-config", Namespace: testNS},
			Reason:         "Updated",
			LastTimestamp:  metav1.Now(),
		},
	)

	res, err := NewKubeResourceService(pool).Get(context.Background(), "test", "", "ConfigMap", testNS, "web-config")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Events) != 1 || res.Events[0].Reason != "Updated" {
		t.Errorf("unexpected events: %+v", res.Events)
	}
}

func TestResourceGetUnknownKindIsNotFound(t *testing.T) {
	_, err := NewKubeResourceService(newTestPool()).Get(context.Background(), "test", "example.com", "Widget", testNS, "x")
	if err == nil || !strings.Contains(err.Error(), "not served by this cluster") {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestResourceGetCustomResource(t *testing.T) {
	res, err := NewKubeResourceService(newTestPool(rollout("web", "ro-web"))).Get(context.Background(), "test", "argoproj.io", "Rollout", testNS, "web")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.YAML, "kind: Rollout") || !strings.Contains(res.YAML, "apiVersion: argoproj.io/v1alpha1") {
		t.Errorf("unexpected yaml:\n%s", res.YAML)
	}
}

func TestPodLimitsTreatsAnyUnlimitedContainerAsUnlimited(t *testing.T) {
	limited := corev1.Container{Name: "a", Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("500m"),
		corev1.ResourceMemory: resource.MustParse("256Mi"),
	}}}
	unlimited := corev1.Container{Name: "b"}

	cpu, mem := podLimits([]corev1.Container{limited, unlimited}, "")
	if cpu != 0 || mem != 0 {
		t.Errorf("expected unlimited pod, got cpu=%d mem=%d", cpu, mem)
	}

	cpu, mem = podLimits([]corev1.Container{limited, unlimited}, "a")
	if cpu != 500 || mem != 256*1024*1024 {
		t.Errorf("expected container a limits, got cpu=%d mem=%d", cpu, mem)
	}
}
