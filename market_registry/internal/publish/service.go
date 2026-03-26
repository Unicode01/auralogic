package publish

import (
	pubpublish "auralogic/market_registry/pkg/publish"

	"auralogic/market_registry/internal/signing"
	"auralogic/market_registry/internal/storage"
)

type Request = pubpublish.Request
type Metadata = pubpublish.Metadata
type Publisher = pubpublish.Publisher
type Options = pubpublish.Options
type RequestError = pubpublish.RequestError
type Service = pubpublish.Service
type RebuildResult = pubpublish.RebuildResult
type DeleteResult = pubpublish.DeleteResult
type SnapshotStatus = pubpublish.SnapshotStatus
type RegistryStatus = pubpublish.RegistryStatus

func NewService(store storage.Storage, sign *signing.Service, keyID string) *Service {
	return pubpublish.NewService(store, sign, keyID)
}

func NewServiceWithOptions(store storage.Storage, sign *signing.Service, keyID string, opts Options) *Service {
	return pubpublish.NewServiceWithOptions(store, sign, keyID, opts)
}

func IsRequestError(err error) bool {
	return pubpublish.IsRequestError(err)
}
