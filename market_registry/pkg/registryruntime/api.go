package registryruntime

import (
	"context"
	"net/http"

	"auralogic/market_registry/pkg/adminapi"
	"auralogic/market_registry/pkg/analytics"
	"auralogic/market_registry/pkg/auth"
	"auralogic/market_registry/pkg/catalog"
	"auralogic/market_registry/pkg/runtimeconfig"
	"auralogic/market_registry/pkg/sourceapi"
)

type API struct {
	addr         string
	dataDir      string
	baseURL      string
	catalogStore *catalog.RuntimeStore
	public       http.Handler
	admin        *adminapi.Handler
}

func NewAPI(cfg runtimeconfig.API) (*API, error) {
	shared, err := newSharedRuntime(cfg.Shared)
	if err != nil {
		return nil, err
	}
	authSvc := auth.NewServiceWithConfig(auth.Config{
		AdminUsername:     cfg.AdminUsername,
		AdminPassword:     cfg.AdminPassword,
		AdminPasswordHash: cfg.AdminPasswordHash,
		TokenTTL:          cfg.TokenTTL,
	})
	statsSvc := analytics.NewService(shared.store, analytics.Config{})
	catalogStore := catalog.NewRuntimeStore(shared.store, shared.signing, catalog.RuntimeStoreConfig{
		SourceID:   cfg.Shared.SourceID,
		SourceName: cfg.Shared.SourceName,
		KeyID:      cfg.Shared.KeyID,
		Settings:   shared.settings,
	})
	publicHandler := sourceapi.NewHandler(sourceapi.Config{
		BaseURL:   cfg.Shared.BaseURL,
		Store:     catalogStore,
		Analytics: statsSvc,
		Registry:  shared.publish,
		AdminUI:   "/admin/ui/",
	})
	adminHandler := adminapi.NewHandlerWithOptions(authSvc, shared.publish, shared.store, shared.signing, adminapi.Options{
		Settings: shared.settings,
	})
	return &API{
		addr:         cfg.Addr,
		dataDir:      cfg.Shared.DataDir,
		baseURL:      cfg.Shared.BaseURL,
		catalogStore: catalogStore,
		public:       publicHandler,
		admin:        adminHandler,
	}, nil
}

func (a *API) Addr() string {
	return a.addr
}

func (a *API) DataDir() string {
	return a.dataDir
}

func (a *API) BaseURL() string {
	return a.baseURL
}

func (a *API) BootstrapSourceDocument(ctx context.Context, baseURL string) (map[string]any, error) {
	return a.catalogStore.BuildSourceDocument(ctx, baseURL)
}

func (a *API) PublicHandler() http.Handler {
	return a.public
}

func (a *API) RegisterAdminRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/admin/auth/login", a.admin.Login)
	mux.HandleFunc("/admin/publish", a.admin.Publish)
	mux.HandleFunc("/admin/sync/github-release/release", a.admin.PreviewGitHubRelease)
	mux.HandleFunc("/admin/sync/github-release/inspect", a.admin.InspectGitHubRelease)
	mux.HandleFunc("/admin/sync/github-release", a.admin.SyncGitHubRelease)
	mux.HandleFunc("/admin/registry/status", a.admin.GetRegistryStatus)
	mux.HandleFunc("/admin/registry/reindex", a.admin.ReindexRegistry)
	mux.HandleFunc("/admin/stats/overview", a.admin.GetStats)
	mux.HandleFunc("/admin/settings", a.admin.HandleSettings)
	mux.HandleFunc("/admin/artifacts", a.admin.ListArtifacts)
	mux.HandleFunc("/admin/artifacts/", a.admin.HandleArtifactRoute)
}
