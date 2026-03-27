package main

import (
	"fmt"
	"log"

	"github.com/codeofmario/wiremap/internal/wiremap/config"
	"github.com/gin-gonic/gin"
)

type App struct {
	router   *gin.Engine
	settings *config.Settings
}

func NewApp(router *gin.Engine, settings *config.Settings) *App {
	return &App{
		router:   router,
		settings: settings,
	}
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
