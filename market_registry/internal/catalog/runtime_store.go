package catalog

import (
	pubcatalog "auralogic/market_registry/pkg/catalog"

	"auralogic/market_registry/internal/signing"
	"auralogic/market_registry/internal/storage"
)

var ErrNotFound = pubcatalog.ErrNotFound

type RuntimeStoreConfig = pubcatalog.RuntimeStoreConfig
type RuntimeStore = pubcatalog.RuntimeStore

func NewRuntimeStore(store storage.Storage, sign *signing.Service, cfg RuntimeStoreConfig) *RuntimeStore {
	return pubcatalog.NewRuntimeStore(store, sign, cfg)
}
