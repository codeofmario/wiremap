package service

import (
	"context"

	"github.com/codeofmario/wiremap/internal/wiremap/dto"
	"github.com/codeofmario/wiremap/internal/wiremap/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// storageGraph is the cluster-wide storage lens: PVC → PersistentVolume → StorageClass.
func (s *kubeTopologyService) storageGraph(ctx context.Context, c *kube.Cluster) (*dto.KubeGraphDto, error) {
	classes, err := c.Clientset.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, kube.WrapError(err, "list storageclasses")
	}
	volumes, err := c.Clientset.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, kube.WrapError(err, "list persistentvolumes")
	}
	claims, err := c.Clientset.CoreV1().PersistentVolumeClaims("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, kube.WrapError(err, "list persistentvolumeclaims")
	}

	g := newGraph(LevelCluster)
	for _, sc := range classes.Items {
		isDefault := "no"
		if sc.Annotations["storageclass.kubernetes.io/is-default-class"] == "true" {
			isDefault = "yes"
		}
		g.addNode(dto.KubeGraphNodeDto{
			ID: kubeNodeID("StorageClass", "", sc.Name), Kind: "StorageClass", Name: sc.Name,
			Status: statusHealthy, Summary: sc.Provisioner, Weight: 1,
			Details: map[string]string{"Default": isDefault},
		})
	}
	for i := range volumes.Items {
		pv := &volumes.Items[i]
		pvID := addPersistentVolume(g, pv)
		if pv.Spec.StorageClassName != "" {
			g.addEdge(pvID, addStorageClass(g, pv.Spec.StorageClassName), "uses")
		}
	}
	for i := range claims.Items {
		pvc := &claims.Items[i]
		status, summary := pvcStatus(pvc)
		claimID := kubeNodeID("PersistentVolumeClaim", pvc.Namespace, pvc.Name)
		g.addNode(dto.KubeGraphNodeDto{
			ID: claimID, Kind: "PersistentVolumeClaim", Name: pvc.Name, Namespace: pvc.Namespace,
			Status: status, Summary: pvc.Namespace + " · " + summary, Drillable: true, Weight: 1,
		})
		pvID := kubeNodeID("PersistentVolume", "", pvc.Spec.VolumeName)
		switch {
		case pvc.Spec.VolumeName != "" && g.hasNode(pvID):
			g.addEdge(claimID, pvID, "bound")
		case pvc.Spec.StorageClassName != nil && *pvc.Spec.StorageClassName != "":
			g.addEdge(claimID, addStorageClass(g, *pvc.Spec.StorageClassName), "uses")
		}
	}
	return g.result(), nil
}
