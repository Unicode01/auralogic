package admin

import (
	"net/http"

	pubadminapi "auralogic/market_registry/pkg/adminapi"

	"auralogic/market_registry/internal/auth"
	"auralogic/market_registry/internal/publish"
	"auralogic/market_registry/internal/signing"
	"auralogic/market_registry/internal/storage"
)

type Handler struct {
	inner   *pubadminapi.Handler
	auth    *auth.Service
	publish *publish.Service
	storage storage.Storage
	signing *signing.Service
}

func NewHandler(authSvc *auth.Service, pubSvc *publish.Service, store storage.Storage, sign *signing.Service) *Handler {
	return NewHandlerWithOptions(authSvc, pubSvc, store, sign, pubadminapi.Options{})
}

func NewHandlerWithOptions(authSvc *auth.Service, pubSvc *publish.Service, store storage.Storage, sign *signing.Service, opts pubadminapi.Options) *Handler {
	return &Handler{
		inner:   pubadminapi.NewHandlerWithOptions(authSvc, pubSvc, store, sign, opts),
		auth:    authSvc,
		publish: pubSvc,
		storage: store,
		signing: sign,
	}
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	h.inner.Login(w, r)
}

func (h *Handler) Publish(w http.ResponseWriter, r *http.Request) {
	h.inner.Publish(w, r)
}

func (h *Handler) SyncGitHubRelease(w http.ResponseWriter, r *http.Request) {
	h.inner.SyncGitHubRelease(w, r)
}

func (h *Handler) PreviewGitHubRelease(w http.ResponseWriter, r *http.Request) {
	h.inner.PreviewGitHubRelease(w, r)
}

func (h *Handler) InspectGitHubRelease(w http.ResponseWriter, r *http.Request) {
	h.inner.InspectGitHubRelease(w, r)
}

func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	h.inner.GetStats(w, r)
}

func (h *Handler) HandleSettings(w http.ResponseWriter, r *http.Request) {
	h.inner.HandleSettings(w, r)
}

func (h *Handler) GetRegistryStatus(w http.ResponseWriter, r *http.Request) {
	h.inner.GetRegistryStatus(w, r)
}

func (h *Handler) ReindexRegistry(w http.ResponseWriter, r *http.Request) {
	h.inner.ReindexRegistry(w, r)
}

func (h *Handler) ListArtifacts(w http.ResponseWriter, r *http.Request) {
	h.inner.ListArtifacts(w, r)
}

func (h *Handler) HandleArtifactRoute(w http.ResponseWriter, r *http.Request) {
	h.inner.HandleArtifactRoute(w, r)
}

func (h *Handler) CheckArtifactOrigins(w http.ResponseWriter, r *http.Request) {
	h.inner.CheckArtifactOrigins(w, r)
}

func (h *Handler) GetArtifactVersions(w http.ResponseWriter, r *http.Request) {
	h.inner.GetArtifactVersions(w, r)
}

func (h *Handler) GetArtifactRelease(w http.ResponseWriter, r *http.Request) {
	h.inner.GetArtifactRelease(w, r)
}

func (h *Handler) DeleteArtifact(w http.ResponseWriter, r *http.Request) {
	h.inner.DeleteArtifact(w, r)
}

func (h *Handler) DeleteArtifactRelease(w http.ResponseWriter, r *http.Request) {
	h.inner.DeleteArtifactRelease(w, r)
}

func (h *Handler) ResyncArtifactVersion(w http.ResponseWriter, r *http.Request) {
	h.inner.ResyncArtifactVersion(w, r)
}

func (h *Handler) CheckArtifactOrigin(w http.ResponseWriter, r *http.Request) {
	h.inner.CheckArtifactOrigin(w, r)
}
