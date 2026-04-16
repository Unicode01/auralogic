package service

import (
	"log"
	"runtime/debug"
)

func recoverBackgroundServicePanic(name string) {
	if recovered := recover(); recovered != nil {
		log.Printf("[panic-guard] %s panic recovered: %v\n%s", name, recovered, debug.Stack())
	}
}
