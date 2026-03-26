package sourceapi

import (
	"net/http"

	pubsourceapi "auralogic/market_registry/pkg/sourceapi"

	"auralogic/market_registry/internal/analytics"
)

type Store = pubsourceapi.Store
type RegistryStatusProvider = pubsourceapi.RegistryStatusProvider
type Config = pubsourceapi.Config

func NewHandler(cfg Config) http.Handler {
	return pubsourceapi.NewHandler(pubsourceapi.Config{
		BaseURL:   cfg.BaseURL,
		Store:     cfg.Store,
		Analytics: (*analytics.Service)(cfg.Analytics),
		Registry:  cfg.Registry,
		AdminUI:   cfg.AdminUI,
	})
}
