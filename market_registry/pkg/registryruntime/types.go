package registryruntime

type Publisher struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Metadata struct {
	Title         string         `json:"title"`
	Summary       string         `json:"summary"`
	Description   string         `json:"description"`
	ReleaseNotes  string         `json:"release_notes"`
	Publisher     Publisher      `json:"publisher"`
	Labels        []string       `json:"labels"`
	Compatibility map[string]any `json:"compatibility,omitempty"`
	Permissions   map[string]any `json:"permissions,omitempty"`
}

type PublishRequest struct {
	Kind                     string
	Name                     string
	Version                  string
	Channel                  string
	ArtifactStorageProfileID string
	ArtifactZip              []byte
	Metadata                 Metadata
}

type GitHubReleaseSyncRequest struct {
	Kind                     string
	Name                     string
	Version                  string
	Channel                  string
	ArtifactStorageProfileID string
	Metadata                 Metadata
	Owner                    string
	Repo                     string
	Tag                      string
	AssetName                string
	APIBaseURL               string
	Token                    string
}

type GitHubReleaseSyncResult struct {
	Kind                     string `json:"kind"`
	Name                     string `json:"name"`
	Version                  string `json:"version"`
	Channel                  string `json:"channel"`
	ArtifactStorageProfileID string `json:"artifact_storage_profile_id,omitempty"`
	Owner                    string `json:"owner"`
	Repo                     string `json:"repo"`
	Tag                      string `json:"tag"`
	AssetName                string `json:"asset_name"`
	ReleaseID                int64  `json:"release_id"`
	AssetID                  int64  `json:"asset_id"`
	AssetSize                int64  `json:"asset_size"`
	SHA256                   string `json:"sha256"`
	APIBaseURL               string `json:"api_base_url"`
	PublishedAt              string `json:"published_at,omitempty"`
	BrowserURL               string `json:"browser_url,omitempty"`
	AssetAPIURL              string `json:"asset_api_url,omitempty"`
	AssetDownloadURL         string `json:"asset_download_url,omitempty"`
}

type KeyPairResult struct {
	KeyID          string `json:"key_id"`
	PublicKey      string `json:"public_key"`
	PrivateKeyPath string `json:"private_key_path"`
}

type RebuildResult struct {
	SourcePath     string `json:"source_path"`
	CatalogPath    string `json:"catalog_path"`
	TotalArtifacts int    `json:"total_artifacts"`
	GeneratedAt    string `json:"generated_at"`
}

type SnapshotStatus struct {
	Path        string   `json:"path"`
	Exists      bool     `json:"exists"`
	Stale       bool     `json:"stale"`
	Status      string   `json:"status"`
	GeneratedAt string   `json:"generatedAt,omitempty"`
	UpdatedAt   string   `json:"updatedAt,omitempty"`
	ItemCount   int      `json:"itemCount,omitempty"`
	Issues      []string `json:"issues,omitempty"`
}

type RegistryStatus struct {
	Healthy       bool           `json:"healthy"`
	Status        string         `json:"status"`
	Message       string         `json:"message"`
	CheckedAt     string         `json:"checkedAt"`
	ArtifactCount int            `json:"artifactCount"`
	Issues        []string       `json:"issues"`
	Source        SnapshotStatus `json:"source"`
	Catalog       SnapshotStatus `json:"catalog"`
	Stats         SnapshotStatus `json:"stats"`
}
