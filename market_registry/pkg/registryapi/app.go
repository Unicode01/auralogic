package registryapi

import (
	"context"
	"log"
	"net/http"

	"auralogic/market_registry/internal/adminui"
	"auralogic/market_registry/pkg/registryruntime"
	"auralogic/market_registry/pkg/runtimeconfig"
)

func Run() error {
	cfg := runtimeconfig.LoadAPI()

	apiRuntime, err := registryruntime.NewAPI(cfg)
	if err != nil {
		return err
	}
	if document, err := apiRuntime.BootstrapSourceDocument(context.Background(), cfg.Shared.BaseURL); err != nil {
		log.Printf("warning: source document bootstrap failed: %v", err)
	} else {
		log.Printf("source registry ready: %s (%s)", document["name"], document["source_id"])
	}

	mux := http.NewServeMux()
	publicHandler := apiRuntime.PublicHandler()
	mux.Handle("/", publicHandler)
	apiRuntime.RegisterAdminRoutes(mux)
	mux.Handle("/admin/ui/", adminui.NewHandler("/admin/ui"))
	mux.Handle("/admin/ui", adminui.RedirectHandler("/admin/ui/"))
	mux.Handle("/admin", adminui.RedirectHandler("/admin/ui/"))

	log.Printf("market-registry api listening on %s", apiRuntime.Addr())
	log.Printf("data dir: %s", apiRuntime.DataDir())
	log.Printf("registry base url: %s", apiRuntime.BaseURL())
	return http.ListenAndServe(apiRuntime.Addr(), enableCORS(mux))
}

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
