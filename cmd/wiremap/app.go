package main

import (
	"fmt"
	"log"

	"github.com/codeofmario/wiremap/internal/wiremap/config"
	"github.com/codeofmario/wiremap/internal/wiremap/docker"
	"github.com/codeofmario/wiremap/internal/wiremap/kube"
	"github.com/gin-gonic/gin"
)

type App struct {
	router   *gin.Engine
	settings *config.Settings
}

func NewApp(router *gin.Engine, settings *config.Settings, dockerPool *docker.ClientPool, clusterPool *kube.ClusterPool) (*App, error) {
	if dockerPool.Connected() == 0 && clusterPool.Connected() == 0 {
		return nil, fmt.Errorf("no Docker hosts or Kubernetes clusters could be connected")
	}

	return &App{
		router:   router,
		settings: settings,
	}, nil
}

func (a *App) Run() error {
	addr := fmt.Sprintf(":%d", a.settings.Port)
	url := fmt.Sprintf("http://localhost:%d", a.settings.Port)

	fmt.Println()
	fmt.Println("  Wiremap is running!")
	fmt.Printf("  Open in browser: %s\n", url)
	fmt.Println()

	if a.settings.DevMode {
		log.Printf("Dev mode enabled — frontend proxied to http://localhost:5173")
	}

	return a.router.Run(addr)
}
