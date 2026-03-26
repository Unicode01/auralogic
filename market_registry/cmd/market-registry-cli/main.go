package main

import (
	"os"

	"auralogic/market_registry/pkg/registrycli"
)

func main() {
	app := registrycli.New()
	os.Exit(app.Run(os.Args))
}

