package admin

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
)

type adminPluginMarketSourceRequest struct {
	SourceID       string   `json:"source_id"`
	Name           string   `json:"name"`
	BaseURL        string   `json:"base_url"`
	PublicKey      string   `json:"public_key"`
	DefaultChannel string   `json:"default_channel"`
	AllowedKinds   []string `json:"allowed_kinds"`
	Enabled        *bool    `json:"enabled"`
}

type adminPluginMarketPreviewRequest struct {
	Source  adminPluginMarketSourceRequest `json:"source"`
	Kind    string                         `json:"kind"`
	Name    string                         `json:"name"`
	Version string                         `json:"version"`
}

type adminPluginMarketInstallRequest struct {
	Source             adminPluginMarketSourceRequest `json:"source"`
	Kind               string                         `json:"kind"`
	Name               string                         `json:"name"`
	Version            string                         `json:"version"`
	GrantedPermissions []string                       `json:"granted_permissions"`
	Activate           *bool                          `json:"activate"`
	AutoStart          *bool                          `json:"auto_start"`
	Note               string                         `json:"note"`
}

func buildAdminPluginMarketSource(input adminPluginMarketSourceRequest) service.PluginMarketSource {
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	return service.PluginMarketSource{
		SourceID:       strings.TrimSpace(input.SourceID),
		Name:           strings.TrimSpace(input.Name),
		BaseURL:        strings.TrimSpace(input.BaseURL),
		PublicKey:      strings.TrimSpace(input.PublicKey),
		DefaultChannel: strings.TrimSpace(input.DefaultChannel),
		AllowedKinds:   normalizeLowerStringListValues(input.AllowedKinds),
		Enabled:        enabled,
	}
}

func (h *PluginHandler) pluginHostRuntime() *service.PluginHostRuntime {
	if h == nil {
		return nil
	}
	return service.NewPluginHostRuntime(h.db, h.pluginManager.Config(), h.pluginManager)
}

func (h *PluginHandler) respondPluginHostActionError(c *gin.Context, err error) {
	var hostErr *service.PluginHostActionError
	if errors.As(err, &hostErr) {
		h.respondPluginError(c, hostErr.Status, hostErr.Message)
		return
	}
	h.respondPluginErrorErr(c, http.StatusBadGateway, err)
}

func (h *PluginHandler) PreviewPluginMarketInstall(c *gin.Context) {
	var req adminPluginMarketPreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondPluginError(c, http.StatusBadRequest, "invalid market preview request")
		return
	}

	result, err := service.PreviewPluginMarketInstallWithSource(
		h.pluginHostRuntime(),
		buildAdminPluginMarketSource(req.Source),
		req.Kind,
		req.Name,
		req.Version,
	)
	if err != nil {
		h.respondPluginHostActionError(c, err)
		return
	}

	h.logPluginOperation(c, "plugin_market_install_preview", nil, nil, map[string]interface{}{
		"source_id": req.Source.SourceID,
		"kind":      strings.TrimSpace(req.Kind),
		"name":      strings.TrimSpace(req.Name),
		"version":   strings.TrimSpace(req.Version),
	})
	c.JSON(http.StatusOK, result)
}

func (h *PluginHandler) InstallPluginFromMarket(c *gin.Context) {
	var req adminPluginMarketInstallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondPluginError(c, http.StatusBadRequest, "invalid market install request")
		return
	}
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	if h.pluginManager != nil {
		originalReq := req
		hookPayload, payloadErr := adminHookStructToPayload(req)
		if payloadErr != nil {
			log.Printf("plugin.market.install.before payload build failed: admin=%d err=%v", adminIDValue, payloadErr)
		} else {
			hookPayload["admin_id"] = adminIDValue
			hookPayload["source"] = "market"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "plugin.market.install.before",
				Payload: hookPayload,
			}, buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "plugin_market",
				"hook_source":   "admin_api",
			}))
			if hookErr != nil {
				log.Printf("plugin.market.install.before hook execution failed: admin=%d source=%s name=%s err=%v", adminIDValue, req.Source.SourceID, req.Name, hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "Market install rejected by plugin"
					}
					h.respondPluginError(c, http.StatusBadRequest, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeAdminHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("plugin.market.install.before payload apply failed, fallback to original request: admin=%d err=%v", adminIDValue, mergeErr)
						req = originalReq
					}
				}
			}
		}
	}
	req.Kind = strings.TrimSpace(req.Kind)
	req.Name = strings.TrimSpace(req.Name)
	req.Version = strings.TrimSpace(req.Version)
	req.Note = strings.TrimSpace(req.Note)
	req.GrantedPermissions = normalizeLowerStringListValues(req.GrantedPermissions)

	params := map[string]interface{}{}
	if req.Activate != nil {
		params["activate"] = *req.Activate
	}
	if req.AutoStart != nil {
		params["auto_start"] = *req.AutoStart
	}
	if note := strings.TrimSpace(req.Note); note != "" {
		params["note"] = note
	}
	if len(req.GrantedPermissions) > 0 {
		params["granted_permissions"] = normalizeLowerStringListValues(req.GrantedPermissions)
	}

	result, err := service.ExecutePluginMarketInstallWithSource(
		h.pluginHostRuntime(),
		buildAdminPluginMarketSource(req.Source),
		req.Kind,
		req.Name,
		req.Version,
		params,
		getOptionalUserID(c),
	)
	if err != nil {
		h.respondPluginHostActionError(c, err)
		return
	}

	h.logPluginOperation(c, "plugin_market_install", nil, nil, map[string]interface{}{
		"source_id":  req.Source.SourceID,
		"kind":       strings.TrimSpace(req.Kind),
		"name":       strings.TrimSpace(req.Name),
		"version":    strings.TrimSpace(req.Version),
		"activate":   req.Activate != nil && *req.Activate,
		"auto_start": req.AutoStart != nil && *req.AutoStart,
	})
	payload := gin.H{"success": true}
	for key, value := range result {
		payload[key] = value
	}
	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"source_id":           req.Source.SourceID,
			"source_name":         req.Source.Name,
			"kind":                req.Kind,
			"name":                req.Name,
			"version":             req.Version,
			"granted_permissions": req.GrantedPermissions,
			"activate":            req.Activate != nil && *req.Activate,
			"auto_start":          req.AutoStart != nil && *req.AutoStart,
			"note":                req.Note,
			"result":              result,
			"admin_id":            adminIDValue,
			"source":              "market",
		}
		go func(execCtx *service.ExecutionContext, hookPayload map[string]interface{}) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "plugin.market.install.after",
				Payload: hookPayload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("plugin.market.install.after hook execution failed: admin=%d source=%s name=%s err=%v", adminIDValue, req.Source.SourceID, req.Name, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "plugin_market",
			"hook_source":   "admin_api",
		})), afterPayload)
	}
	c.JSON(http.StatusOK, payload)
}
