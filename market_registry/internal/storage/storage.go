package storage

import pubstorage "auralogic/market_registry/pkg/storage"

type Storage = pubstorage.Storage
type Config = pubstorage.Config

func New(cfg Config) (Storage, error) {
	return pubstorage.New(cfg)
}
