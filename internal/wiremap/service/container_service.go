package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/codeofmario/wiremap/internal/wiremap/docker"
	"github.com/codeofmario/wiremap/internal/wiremap/dto"
	apperrors "github.com/codeofmario/wiremap/internal/wiremap/errors"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type ExecSession struct {
	Conn io.ReadWriteCloser
}

type ContainerService interface {
	List(ctx context.Context, hostID string) ([]dto.ContainerListItem, error)
	Inspect(ctx context.Context, hostID string, id string) (*dto.ContainerInspectDto, error)
	Logs(ctx context.Context, hostID string, id string, tail string) (io.ReadCloser, error)
	Stats(ctx context.Context, hostID string, id string) (io.ReadCloser, error)
	ParseStats(raw json.RawMessage) (*dto.StatsDto, error)
	Exec(ctx context.Context, hostID string, id string, cmd []string) (*ExecSession, error)
	ExecResize(ctx context.Context, hostID string, execID string, height, width uint) error
	UpdateEnv(ctx context.Context, hostID string, id string, env []string) (string, error)
}

type containerService struct {
	pool *docker.ClientPool
}

func NewContainerService(pool *docker.ClientPool) ContainerService {
	return &containerService{pool: pool}
}

func (s *containerService) List(ctx context.Context, hostID string) ([]dto.ContainerListItem, error) {
	c, err := s.pool.Get(hostID)
	if err != nil {
		return nil, err
	}

	containers, err := c.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, apperrors.Internal(fmt.Sprintf("failed to list containers: %s", err))
	}

	result := make([]dto.ContainerListItem, 0, len(containers))
	for _, ct := range containers {
		item := dto.ContainerListItem{
			ID:      ct.ID,
			Names:   ct.Names,
			Image:   ct.Image,
			State:   ct.State,
			Status:  ct.Status,
			Created: ct.Created,
			Labels:  ct.Labels,
		}

		item.Ports = make([]dto.PortMappingDto, 0, len(ct.Ports))
		for _, p := range ct.Ports {
			item.Ports = append(item.Ports, dto.PortMappingDto{
				IP:          p.IP,
				PrivatePort: p.PrivatePort,
				PublicPort:  p.PublicPort,
				Type:        p.Type,
			})
		}

		item.Networks = make(map[string]dto.NetworkAttachment)
		if ct.NetworkSettings != nil {
			for name, net := range ct.NetworkSettings.Networks {
				item.Networks[name] = dto.NetworkAttachment{
					IPAddress:  net.IPAddress,
					Gateway:    net.Gateway,
					MacAddress: net.MacAddress,
					NetworkID:  net.NetworkID,
					Aliases:    net.Aliases,
				}
			}
		}

		result = append(result, item)
	}

	return result, nil
}

func (s *containerService) Inspect(ctx context.Context, hostID string, id string) (*dto.ContainerInspectDto, error) {
	c, err := s.pool.Get(hostID)
	if err != nil {
		return nil, err
	}

	info, err := c.ContainerInspect(ctx, id)
	if err != nil {
		if client.IsErrNotFound(err) {
			return nil, apperrors.NotFound(fmt.Sprintf("container %s not found", id))
		}
		return nil, apperrors.Internal(fmt.Sprintf("failed to inspect container: %s", err))
	}

	result := &dto.ContainerInspectDto{
		ID:      info.ID,
		Name:    strings.TrimPrefix(info.Name, "/"),
		Image:   info.Config.Image,
		Created: info.Created,
		State: dto.ContainerStateDto{
			Status:     info.State.Status,
			Running:    info.State.Running,
			Paused:     info.State.Paused,
			Restarting: info.State.Restarting,
			StartedAt:  info.State.StartedAt,
			FinishedAt: info.State.FinishedAt,
		},
		Labels: info.Config.Labels,
	}

	if info.State.Health != nil {
		result.State.Health = info.State.Health.Status
	}

	if info.Config.Cmd != nil {
		result.Command = strings.Join(info.Config.Cmd, " ")
	}
	result.Entrypoint = info.Config.Entrypoint
	result.Env = info.Config.Env

	if info.HostConfig != nil {
		result.RestartPolicy = string(info.HostConfig.RestartPolicy.Name)

		result.PortBindings = make(map[string][]dto.HostBinding)
		for port, bindings := range info.HostConfig.PortBindings {
			hbs := make([]dto.HostBinding, 0, len(bindings))
			for _, b := range bindings {
				hbs = append(hbs, dto.HostBinding{HostIP: b.HostIP, HostPort: b.HostPort})
			}
			result.PortBindings[string(port)] = hbs
		}
	}

	result.Mounts = make([]dto.MountDto, 0, len(info.Mounts))
	for _, m := range info.Mounts {
		result.Mounts = append(result.Mounts, dto.MountDto{
			Type:        string(m.Type),
			Name:        m.Name,
			Source:      m.Source,
			Destination: m.Destination,
			Mode:        m.Mode,
			RW:          m.RW,
		})
	}

	result.Networks = make(map[string]dto.NetworkAttachment)
	if info.NetworkSettings != nil {
		for name, net := range info.NetworkSettings.Networks {
			result.Networks[name] = dto.NetworkAttachment{
				IPAddress:  net.IPAddress,
				Gateway:    net.Gateway,
				MacAddress: net.MacAddress,
				NetworkID:  net.NetworkID,
				Aliases:    net.Aliases,
			}
		}
	}

	return result, nil
}

func (s *containerService) Logs(ctx context.Context, hostID string, id string, tail string) (io.ReadCloser, error) {
	c, err := s.pool.Get(hostID)
	if err != nil {
		return nil, err
	}

	if tail == "" {
		tail = "100"
	}

	reader, err := c.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: true,
		Tail:       tail,
	})
	if err != nil {
		return nil, apperrors.Internal(fmt.Sprintf("failed to get logs: %s", err))
	}

	return reader, nil
}

func (s *containerService) Stats(ctx context.Context, hostID string, id string) (io.ReadCloser, error) {
	c, err := s.pool.Get(hostID)
	if err != nil {
		return nil, err
	}

	stats, err := c.ContainerStats(ctx, id, true)
	if err != nil {
		return nil, apperrors.Internal(fmt.Sprintf("failed to get stats: %s", err))
	}

	return stats.Body, nil
}

func (s *containerService) ParseStats(raw json.RawMessage) (*dto.StatsDto, error) {
	var stats types.StatsJSON
	if err := json.Unmarshal(raw, &stats); err != nil {
		return nil, err
	}

	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)
	cpuCount := float64(stats.CPUStats.OnlineCPUs)
	if cpuCount == 0 {
		cpuCount = 1
	}

	var cpuPercent float64
	if systemDelta > 0 {
		cpuPercent = (cpuDelta / systemDelta) * cpuCount * 100
	}

	memUsage := stats.MemoryStats.Usage
	memLimit := stats.MemoryStats.Limit
	var memPercent float64
	if memLimit > 0 {
		memPercent = float64(memUsage) / float64(memLimit) * 100
	}

	var netRx, netTx uint64
	for _, net := range stats.Networks {
		netRx += net.RxBytes
		netTx += net.TxBytes
	}

	return &dto.StatsDto{
		CPUPercent:    math.Round(cpuPercent*100) / 100,
		MemoryUsage:   memUsage,
		MemoryLimit:   memLimit,
		MemoryPercent: math.Round(memPercent*100) / 100,
		NetworkRx:     netRx,
		NetworkTx:     netTx,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func (s *containerService) Exec(ctx context.Context, hostID string, id string, cmd []string) (*ExecSession, error) {
	c, err := s.pool.Get(hostID)
	if err != nil {
		return nil, err
	}

	if len(cmd) == 0 {
		cmd = []string{"/bin/sh"}
	}

	execConfig := types.ExecConfig{
		Cmd:          cmd,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
	}

	exec, err := c.ContainerExecCreate(ctx, id, execConfig)
	if err != nil {
		return nil, apperrors.Internal(fmt.Sprintf("failed to create exec: %s", err))
	}

	resp, err := c.ContainerExecAttach(ctx, exec.ID, types.ExecStartCheck{Tty: true})
	if err != nil {
		return nil, apperrors.Internal(fmt.Sprintf("failed to attach exec: %s", err))
	}

	return &ExecSession{Conn: resp.Conn}, nil
}

func (s *containerService) ExecResize(ctx context.Context, hostID string, execID string, height, width uint) error {
	c, err := s.pool.Get(hostID)
	if err != nil {
		return err
	}

	return c.ContainerExecResize(ctx, execID, types.ResizeOptions{
		Height: height,
		Width:  width,
	})
}

func (s *containerService) UpdateEnv(ctx context.Context, hostID string, id string, env []string) (string, error) {
	c, err := s.pool.Get(hostID)
	if err != nil {
		return "", err
	}

	info, err := c.ContainerInspect(ctx, id)
	if err != nil {
		return "", apperrors.Internal(fmt.Sprintf("failed to inspect container: %s", err))
	}

	// Stop the container
	timeout := 10
	stopOpts := container.StopOptions{Timeout: &timeout}
	if err := c.ContainerStop(ctx, id, stopOpts); err != nil {
		return "", apperrors.Internal(fmt.Sprintf("failed to stop container: %s", err))
	}

	// Remove the container
	if err := c.ContainerRemove(ctx, id, container.RemoveOptions{}); err != nil {
		return "", apperrors.Internal(fmt.Sprintf("failed to remove container: %s", err))
	}

	// Update env in config
	config := info.Config
	config.Env = env

	// Recreate with same config
	resp, err := c.ContainerCreate(ctx, config, info.HostConfig, nil, nil, strings.TrimPrefix(info.Name, "/"))
	if err != nil {
		return "", apperrors.Internal(fmt.Sprintf("failed to recreate container: %s", err))
	}

	// Reconnect to networks
	for name, netConfig := range info.NetworkSettings.Networks {
		if err := c.NetworkConnect(ctx, netConfig.NetworkID, resp.ID, netConfig); err != nil {
			// Log but don't fail — container was already created
			fmt.Printf("warning: failed to reconnect to network %s: %s\n", name, err)
		}
	}

	// Start the container
	if err := c.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", apperrors.Internal(fmt.Sprintf("failed to start container: %s", err))
	}

	return resp.ID, nil
}
