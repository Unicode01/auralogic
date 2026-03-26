package main

import (
	"log"

	"auralogic/market_registry/pkg/registryapi"
)

func main() {
	if err := registryapi.Run(); err != nil {
		log.Fatalf("market-registry-api stopped: %v", err)
	}
}

