//go:build wireinject
// +build wireinject

package main

import (
	"github.com/codeofmario/wiremap/internal/wiremap/config"
	"github.com/codeofmario/wiremap/internal/wiremap/docker"
	"github.com/codeofmario/wiremap/internal/wiremap/handler"
	"github.com/codeofmario/wiremap/internal/wiremap/router"
	"github.com/codeofmario/wiremap/internal/wiremap/service"
	"github.com/codeofmario/wiremap/internal/wiremap/ws"
	"github.com/google/wire"
)

func InitializeApp(port int, devMode bool, configFile string, hosts []string) (*App, error) {
	wire.Build(
		config.NewSettings,

		docker.NewClientPool,

		service.NewContainerService,
		service.NewNetworkService,
		service.NewFilesystemService,

		handler.NewContainerHandler,
		handler.NewNetworkHandler,
		handler.NewFilesystemHandler,

		ws.NewHub,

		router.InitRoutes,

		NewApp,
	)
	return nil, nil
}
