package registryruntime

import (
	"context"
	"path/filepath"
	"strings"

	"auralogic/market_registry/pkg/artifactsync"
	"auralogic/market_registry/pkg/publish"
	"auralogic/market_registry/pkg/registrysettings"
	"auralogic/market_registry/pkg/runtimeconfig"
	"auralogic/market_registry/pkg/signing"
	"auralogic/market_registry/pkg/storage"
)

type sharedRuntime struct {
	store    storage.Storage
	signing  *signing.Service
	publish  *publish.Service
	settings *registrysettings.Service
	shared   runtimeconfig.Shared
}

func newSharedRuntime(cfg runtimeconfig.Shared) (*sharedRuntime, error) {
	store, err := storage.New(storage.Config{
		Type:              cfg.StorageType,
		BaseDir:           cfg.DataDir,
		BaseURL:           cfg.BaseURL,
		S3Endpoint:        cfg.StorageS3Endpoint,
		S3Region:          cfg.StorageS3Region,
		S3Bucket:          cfg.StorageS3Bucket,
		S3Prefix:          cfg.StorageS3Prefix,
		S3AccessKeyID:     cfg.StorageS3AccessKeyID,
		S3SecretAccessKey: cfg.StorageS3SecretAccessKey,
		S3SessionToken:    cfg.StorageS3SessionToken,
		S3UsePathStyle:    cfg.StorageS3UsePathStyle,
		WebDAVEndpoint:    cfg.StorageWebDAVEndpoint,
		WebDAVUsername:    cfg.StorageWebDAVUsername,
		WebDAVPassword:    cfg.StorageWebDAVPassword,
		WebDAVSkipVerify:  cfg.StorageWebDAVSkipVerify,
		FTPAddress:        cfg.StorageFTPAddress,
		FTPUsername:       cfg.StorageFTPUsername,
		FTPPassword:       cfg.StorageFTPPassword,
		FTPRootDir:        cfg.StorageFTPRootDir,
		FTPSecurity:       cfg.StorageFTPSecurity,
		FTPSkipVerify:     cfg.StorageFTPSkipVerify,
	})
	if err != nil {
		return nil, err
	}
	sign := signing.NewService(cfg.KeyDir)
	settings := registrysettings.NewService(store, registrysettings.ArtifactStorageProfile{
		ID:                registrysettings.CanonicalArtifactStorageProfileID,
		Name:              "Canonical Storage",
		Type:              firstNonEmpty(cfg.StorageType, "local"),
		Description:       "Registry metadata storage and default artifact payload storage",
		BaseDir:           cfg.DataDir,
		BaseURL:           cfg.BaseURL,
		S3Endpoint:        cfg.StorageS3Endpoint,
		S3Region:          cfg.StorageS3Region,
		S3Bucket:          cfg.StorageS3Bucket,
		S3Prefix:          cfg.StorageS3Prefix,
		S3AccessKeyID:     cfg.StorageS3AccessKeyID,
		S3SecretAccessKey: cfg.StorageS3SecretAccessKey,
		S3SessionToken:    cfg.StorageS3SessionToken,
		S3UsePathStyle:    cfg.StorageS3UsePathStyle,
		WebDAVEndpoint:    cfg.StorageWebDAVEndpoint,
		WebDAVUsername:    cfg.StorageWebDAVUsername,
		WebDAVPassword:    cfg.StorageWebDAVPassword,
		WebDAVSkipVerify:  cfg.StorageWebDAVSkipVerify,
		FTPAddress:        cfg.StorageFTPAddress,
		FTPUsername:       cfg.StorageFTPUsername,
		FTPPassword:       cfg.StorageFTPPassword,
		FTPRootDir:        cfg.StorageFTPRootDir,
		FTPSecurity:       cfg.StorageFTPSecurity,
		FTPSkipVerify:     cfg.StorageFTPSkipVerify,
	})
	pubSvc := publish.NewServiceWithOptions(store, sign, cfg.KeyID, publish.Options{
		SourceID:   cfg.SourceID,
		SourceName: cfg.SourceName,
		BaseURL:    cfg.BaseURL,
		Settings:   settings,
	})
	return &sharedRuntime{
		store:    store,
		signing:  sign,
		publish:  pubSvc,
		settings: settings,
		shared:   cfg,
	}, nil
}

func (r *sharedRuntime) GenerateKeyPair(keyID string) (KeyPairResult, error) {
	if _, err := r.signing.GenerateKeyPair(keyID); err != nil {
		return KeyPairResult{}, err
	}
	publicKey, err := r.signing.ExportPublicKey(keyID)
	if err != nil {
		return KeyPairResult{}, err
	}
	return KeyPairResult{
		KeyID:          keyID,
		PublicKey:      publicKey,
		PrivateKeyPath: filepath.Join(r.shared.KeyDir, keyID+".key"),
	}, nil
}

func (r *sharedRuntime) Publish(ctx context.Context, req PublishRequest) error {
	return r.publish.Publish(ctx, publish.Request{
		Kind:                     req.Kind,
		Name:                     req.Name,
		Version:                  req.Version,
		Channel:                  req.Channel,
		ArtifactStorageProfileID: req.ArtifactStorageProfileID,
		ArtifactZip:              req.ArtifactZip,
		Metadata: publish.Metadata{
			Title:        req.Metadata.Title,
			Summary:      req.Metadata.Summary,
			Description:  req.Metadata.Description,
			ReleaseNotes: req.Metadata.ReleaseNotes,
			Publisher: publish.Publisher{
				ID:   req.Metadata.Publisher.ID,
				Name: req.Metadata.Publisher.Name,
			},
			Labels:        append([]string(nil), req.Metadata.Labels...),
			Compatibility: cloneMap(req.Metadata.Compatibility),
			Permissions:   cloneMap(req.Metadata.Permissions),
		},
	})
}

func (r *sharedRuntime) SyncGitHubRelease(ctx context.Context, req GitHubReleaseSyncRequest) (GitHubReleaseSyncResult, error) {
	result, err := artifactsync.NewGitHubReleaseSyncer(r.publish).Sync(ctx, artifactsync.GitHubReleaseRequest{
		Kind:                     req.Kind,
		Name:                     req.Name,
		Version:                  req.Version,
		Channel:                  req.Channel,
		ArtifactStorageProfileID: req.ArtifactStorageProfileID,
		Owner:                    req.Owner,
		Repo:                     req.Repo,
		Tag:                      req.Tag,
		AssetName:                req.AssetName,
		APIBaseURL:               req.APIBaseURL,
		Token:                    req.Token,
		Metadata: publish.Metadata{
			Title:        req.Metadata.Title,
			Summary:      req.Metadata.Summary,
			Description:  req.Metadata.Description,
			ReleaseNotes: req.Metadata.ReleaseNotes,
			Publisher: publish.Publisher{
				ID:   req.Metadata.Publisher.ID,
				Name: req.Metadata.Publisher.Name,
			},
			Labels:        append([]string(nil), req.Metadata.Labels...),
			Compatibility: cloneMap(req.Metadata.Compatibility),
			Permissions:   cloneMap(req.Metadata.Permissions),
		},
	})
	if err != nil {
		return GitHubReleaseSyncResult{}, err
	}
	return GitHubReleaseSyncResult{
		Kind:                     result.Kind,
		Name:                     result.Name,
		Version:                  result.Version,
		Channel:                  result.Channel,
		ArtifactStorageProfileID: result.ArtifactStorageProfileID,
		Owner:                    result.Owner,
		Repo:                     result.Repo,
		Tag:                      result.Tag,
		AssetName:                result.AssetName,
		ReleaseID:                result.ReleaseID,
		AssetID:                  result.AssetID,
		AssetSize:                result.AssetSize,
		SHA256:                   result.SHA256,
		APIBaseURL:               result.APIBaseURL,
		PublishedAt:              result.PublishedAt,
		BrowserURL:               result.BrowserURL,
		AssetAPIURL:              result.AssetAPIURL,
		AssetDownloadURL:         result.AssetDownloadURL,
	}, nil
}

func (r *sharedRuntime) RebuildRegistry(ctx context.Context) (RebuildResult, error) {
	result, err := r.publish.RebuildRegistry(ctx)
	if err != nil {
		return RebuildResult{}, err
	}
	return RebuildResult{
		SourcePath:     result.SourcePath,
		CatalogPath:    result.CatalogPath,
		TotalArtifacts: result.TotalArtifacts,
		GeneratedAt:    result.GeneratedAt,
	}, nil
}

func (r *sharedRuntime) RegistryStatus(ctx context.Context) (RegistryStatus, error) {
	status, err := r.publish.RegistryStatus(ctx)
	if err != nil {
		return RegistryStatus{}, err
	}
	return RegistryStatus{
		Healthy:       status.Healthy,
		Status:        status.Status,
		Message:       status.Message,
		CheckedAt:     status.CheckedAt,
		ArtifactCount: status.ArtifactCount,
		Issues:        append([]string(nil), status.Issues...),
		Source:        convertSnapshotStatus(status.Source),
		Catalog:       convertSnapshotStatus(status.Catalog),
		Stats:         convertSnapshotStatus(status.Stats),
	}, nil
}

func convertSnapshotStatus(status publish.SnapshotStatus) SnapshotStatus {
	return SnapshotStatus{
		Path:        status.Path,
		Exists:      status.Exists,
		Stale:       status.Stale,
		Status:      status.Status,
		GeneratedAt: status.GeneratedAt,
		UpdatedAt:   status.UpdatedAt,
		ItemCount:   status.ItemCount,
		Issues:      append([]string(nil), status.Issues...),
	}
}

func cloneMap(value map[string]any) map[string]any {
	if len(value) == 0 {
		return nil
	}
	out := make(map[string]any, len(value))
	for key, item := range value {
		out[key] = item
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
