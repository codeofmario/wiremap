//go:build wireinject
// +build wireinject

package main

import (
	"github.com/codeofmario/wiremap/internal/wiremap/config"
	"github.com/codeofmario/wiremap/internal/wiremap/docker"
	"github.com/codeofmario/wiremap/internal/wiremap/handler"
	"github.com/codeofmario/wiremap/internal/wiremap/kube"
	"github.com/codeofmario/wiremap/internal/wiremap/router"
	"github.com/codeofmario/wiremap/internal/wiremap/service"
	"github.com/codeofmario/wiremap/internal/wiremap/ws"
	"github.com/google/wire"
)

var dockerSet = wire.NewSet(
	docker.NewClientPool,

	service.NewContainerService,
	service.NewNetworkService,
	service.NewFilesystemService,

	handler.NewContainerHandler,
	handler.NewNetworkHandler,
	handler.NewFilesystemHandler,

	ws.NewHub,
)

var kubeSet = wire.NewSet(
	kube.NewClusterPool,

	service.NewKubeClusterService,
	service.NewKubeTopologyService,
	service.NewKubeResourceService,
	service.NewKubePodService,
	service.NewKubeBrowserService,
	service.NewKubeActionService,
	service.NewKubeNodeActionService,
	service.NewKubeWatchService,

	handler.NewKubeClusterHandler,
	handler.NewKubeResourceHandler,
	handler.NewKubePodHandler,
	handler.NewKubeBrowserHandler,
	handler.NewKubeActionHandler,

	ws.NewKubeHub,
)

func InitializeApp(flags config.Flags) (*App, error) {
	wire.Build(
		config.NewSettings,

		dockerSet,
		kubeSet,

		router.InitRoutes,

		NewApp,
	)
	return nil, nil
}
