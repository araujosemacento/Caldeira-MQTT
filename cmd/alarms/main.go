package main

import (
	"embed"
	"flag"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend
var assets embed.FS

func main() {
	broker := flag.String("broker", "tcp://localhost:1883", "URL do broker MQTT")
	flag.Parse()

	app := NewApp(*broker)

	err := wails.Run(&options.App{
		Title:  "Alarms: Caldeira",
		Width:  320,
		Height: 260,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		log.Fatalf("Erro ao iniciar a aplicação Alarms: %v", err)
	}
}
