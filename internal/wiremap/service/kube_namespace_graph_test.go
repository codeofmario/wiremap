package service

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestPodSpecRefsFindsEveryConfigSource(t *testing.T) {
	cmRef := func(name string) corev1.LocalObjectReference { return corev1.LocalObjectReference{Name: name} }
	spec := corev1.PodSpec{
		Volumes: []corev1.Volume{
			{Name: "cm", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: cmRef("settings")}}},
			{Name: "tls", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "tls"}}},
			{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "data"}}},
			{Name: "bundle", VolumeSource: corev1.VolumeSource{Projected: &corev1.ProjectedVolumeSource{Sources: []corev1.VolumeProjection{
				{ConfigMap: &corev1.ConfigMapProjection{LocalObjectReference: cmRef("ca")}},
				{Secret: &corev1.SecretProjection{LocalObjectReference: cmRef("token")}},
			}}}},
			{Name: "scratch", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
		},
		ImagePullSecrets: []corev1.LocalObjectReference{cmRef("registry")},
		InitContainers: []corev1.Container{{
			EnvFrom: []corev1.EnvFromSource{{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: cmRef("init-env")}}},
		}},
		Containers: []corev1.Container{{
			EnvFrom: []corev1.EnvFromSource{{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: cmRef("settings")}}},
			Env: []corev1.EnvVar{
				{Name: "PLAIN", Value: "x"},
				{Name: "FROM_CM", ValueFrom: &corev1.EnvVarSource{ConfigMapKeyRef: &corev1.ConfigMapKeySelector{LocalObjectReference: cmRef("flags")}}},
				{Name: "FROM_SECRET", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: cmRef("db")}}},
				{Name: "FIELD", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}}},
			},
		}},
	}

	want := []specRef{
		{"ConfigMap", "settings"}, {"Secret", "tls"}, {"PersistentVolumeClaim", "data"},
		{"ConfigMap", "ca"}, {"Secret", "token"}, {"Secret", "registry"},
		{"Secret", "init-env"}, {"ConfigMap", "flags"}, {"Secret", "db"},
	}
	if got := podSpecRefs(spec); !reflect.DeepEqual(got, want) {
		t.Errorf("podSpecRefs =\n%v\nwant\n%v", got, want)
	}
}
