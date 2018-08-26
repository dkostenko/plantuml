package main

import (
	"log"
	"os"

	"github.com/dkostenko/plantuml"
	"github.com/dkostenko/plantuml/api"
	"github.com/jawher/mow.cli"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func main() {
	app := cli.App("plantuml", "PlantUML client application.")
	serverAddr := app.StringOpt("plantuml-server-addr", "", "PlantUML server address.")
	apiAddr := app.StringOpt("api-addr", "", "PlantUML UI API address.")

	// Default action: run server with API and UI for using PlantUML server.
	app.Action = func() {
		plantumlManager, err := plantuml.NewManager(*serverAddr)
		if err != nil {
			log.Fatalln(err)
		}

		apiManager := api.NewManager(plantumlManager)
		log.Fatalln(apiManager.Listen(*apiAddr))
	}
	app.Run(os.Args)
}
