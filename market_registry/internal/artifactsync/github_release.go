package artifactsync

import (
	pubsync "auralogic/market_registry/pkg/artifactsync"

	"auralogic/market_registry/internal/publish"
)

type GitHubReleaseRequest = pubsync.GitHubReleaseRequest
type GitHubReleaseResult = pubsync.GitHubReleaseResult
type GitHubReleaseInspectionRequest = pubsync.GitHubReleaseInspectionRequest
type GitHubReleaseInspectionResult = pubsync.GitHubReleaseInspectionResult
type GitHubReleasePreviewRequest = pubsync.GitHubReleasePreviewRequest
type GitHubReleasePreviewResult = pubsync.GitHubReleasePreviewResult
type GitHubReleasePreviewAsset = pubsync.GitHubReleasePreviewAsset
type GitHubReleaseSyncer = pubsync.GitHubReleaseSyncer

func NewGitHubReleaseSyncer(pub *publish.Service) *GitHubReleaseSyncer {
	return pubsync.NewGitHubReleaseSyncer(pub)
}
