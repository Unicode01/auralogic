package main

import (
	"log"
	"os"

	"auralogic/internal/jsworker"
)

func main() {
	if err := jsworker.Run(os.Args[1:]); err != nil {
		log.Fatalf("jsworker: %v", err)
	}
}
