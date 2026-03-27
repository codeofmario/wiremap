package service

import (
	"context"
	"fmt"

	"github.com/codeofmario/wiremap/internal/wiremap/docker"
	"github.com/codeofmario/wiremap/internal/wiremap/dto"
	apperrors "github.com/codeofmario/wiremap/internal/wiremap/errors"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

type NetworkService interface {
	List(ctx context.Context, hostID string) ([]dto.NetworkDto, error)
	Inspect(ctx context.Context, hostID string, id string) (*dto.NetworkDto, error)
}

type networkService struct {
	pool *docker.ClientPool
}

func NewNetworkService(pool *docker.ClientPool) NetworkService {
	return &networkService{pool: pool}
}

func (s *networkService) List(ctx context.Context, hostID string) ([]dto.NetworkDto, error) {
	c, err := s.pool.Get(hostID)
	if err != nil {
		return nil, err
	}

	networks, err := c.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return nil, apperrors.Internal(fmt.Sprintf("failed to list networks: %s", err))
	}

	result := make([]dto.NetworkDto, 0, len(networks))
	for _, n := range networks {
		// NetworkList doesn't populate Containers, so inspect each
		inspected, err := c.NetworkInspect(ctx, n.ID, network.InspectOptions{})
		if err != nil {
			result = append(result, s.toDto(n))
			continue
		}
		result = append(result, s.toDto(inspected))
	}

	return result, nil
}

func (s *networkService) Inspect(ctx context.Context, hostID string, id string) (*dto.NetworkDto, error) {
	c, err := s.pool.Get(hostID)
	if err != nil {
		return nil, err
	}

	n, err := c.NetworkInspect(ctx, id, network.InspectOptions{})
	if err != nil {
		if client.IsErrNotFound(err) {
			return nil, apperrors.NotFound(fmt.Sprintf("network %s not found", id))
		}
		return nil, apperrors.Internal(fmt.Sprintf("failed to inspect network: %s", err))
	}

	result := s.toDto(n)
	return &result, nil
}

func (s *networkService) toDto(n network.Inspect) dto.NetworkDto {
	d := dto.NetworkDto{
		ID:     n.ID,
		Name:   n.Name,
		Driver: n.Driver,
		Scope:  n.Scope,
	}

	if len(n.IPAM.Config) > 0 {
		d.Subnet = n.IPAM.Config[0].Subnet
		d.Gateway = n.IPAM.Config[0].Gateway
	}

	d.Containers = make(map[string]dto.NetworkContainerDto)
	for id, ep := range n.Containers {
		d.Containers[id] = dto.NetworkContainerDto{
			Name:        ep.Name,
			IPv4Address: ep.IPv4Address,
			MacAddress:  ep.MacAddress,
		}
	}

	return d
}
