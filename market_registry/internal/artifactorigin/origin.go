package artifactorigin

import puborigin "auralogic/market_registry/pkg/artifactorigin"

const (
	ProviderLocal         = puborigin.ProviderLocal
	ProviderHTTP          = puborigin.ProviderHTTP
	ProviderGitHubRelease = puborigin.ProviderGitHubRelease
	ProviderGitHubArchive = puborigin.ProviderGitHubArchive
	ProviderWebDAV        = puborigin.ProviderWebDAV
	ProviderS3            = puborigin.ProviderS3

	ModeMirror   = puborigin.ModeMirror
	ModeProxy    = puborigin.ModeProxy
	ModeRedirect = puborigin.ModeRedirect

	CacheStatusReady = puborigin.CacheStatusReady

	SyncStrategyManual   = puborigin.SyncStrategyManual
	SyncStrategyInterval = puborigin.SyncStrategyInterval
	SyncStrategyWebhook  = puborigin.SyncStrategyWebhook
	SyncStrategyLazy     = puborigin.SyncStrategyLazy
)

type Document = puborigin.Document
type Integrity = puborigin.Integrity
type SyncState = puborigin.SyncState
type CacheState = puborigin.CacheState
type ResolveResult = puborigin.ResolveResult
type Resolver = puborigin.Resolver
type Registry = puborigin.Registry
type LocalResolver = puborigin.LocalResolver
type HTTPResolver = puborigin.HTTPResolver

var NormalizeDocument = puborigin.NormalizeDocument
var FromMap = puborigin.FromMap
var TransportMap = puborigin.TransportMap
var DefaultLocalMirrorTransport = puborigin.DefaultLocalMirrorTransport
var DocumentPath = puborigin.DocumentPath

func NewRegistry(resolvers ...Resolver) *Registry {
	return puborigin.NewRegistry(resolvers...)
}

func NewDefaultRegistry() *Registry {
	return puborigin.NewDefaultRegistry()
}

func DefaultLocalMirrorOrigin(artifactPath string, sha256 string, size int64) Document {
	return puborigin.DefaultLocalMirrorOrigin(artifactPath, sha256, size)
}
