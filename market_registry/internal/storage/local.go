package storage

import pubstorage "auralogic/market_registry/pkg/storage"

type LocalStorage = pubstorage.LocalStorage
type S3Storage = pubstorage.S3Storage
type WebDAVStorage = pubstorage.WebDAVStorage
type FTPStorage = pubstorage.FTPStorage

func NewLocalStorage(baseDir, baseURL string) (*LocalStorage, error) {
	return pubstorage.NewLocalStorage(baseDir, baseURL)
}

func NewS3Storage(cfg Config) (*S3Storage, error) {
	return pubstorage.NewS3Storage(cfg)
}

func NewWebDAVStorage(cfg Config) (*WebDAVStorage, error) {
	return pubstorage.NewWebDAVStorage(cfg)
}

func NewFTPStorage(cfg Config) (*FTPStorage, error) {
	return pubstorage.NewFTPStorage(cfg)
}
