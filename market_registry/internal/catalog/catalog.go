package catalog

import pubcatalog "auralogic/market_registry/pkg/catalog"

type Store = pubcatalog.Store
type Query = pubcatalog.Query

func NewDefaultStore() (*Store, error) {
	return pubcatalog.NewDefaultStore()
}
