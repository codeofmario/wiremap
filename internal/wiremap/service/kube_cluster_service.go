package service

import (
	"github.com/codeofmario/wiremap/internal/wiremap/dto"
	"github.com/codeofmario/wiremap/internal/wiremap/kube"
)

type KubeClusterService interface {
	List() []dto.KubeClusterDto
}

type kubeClusterService struct {
	pool *kube.ClusterPool
}

func NewKubeClusterService(pool *kube.ClusterPool) KubeClusterService {
	return &kubeClusterService{pool: pool}
}

func (s *kubeClusterService) List() []dto.KubeClusterDto {
	configs := s.pool.Clusters()
	result := make([]dto.KubeClusterDto, 0, len(configs))
	for _, cfg := range configs {
		result = append(result, dto.KubeClusterDto{Name: cfg.Name, Connected: s.pool.IsConnected(cfg.Name)})
	}
	return result
}
