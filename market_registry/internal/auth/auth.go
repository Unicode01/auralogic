package auth

import (
	pubauth "auralogic/market_registry/pkg/auth"
)

type User = pubauth.User
type Config = pubauth.Config
type Service = pubauth.Service

func NewService() *Service {
	return pubauth.NewService()
}

func NewServiceWithConfig(cfg Config) *Service {
	return pubauth.NewServiceWithConfig(cfg)
}
