package admin

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"auralogic/internal/config"
	"auralogic/internal/middleware"
	"auralogic/internal/models"
	"auralogic/internal/pkg/logger"
	"auralogic/internal/pkg/response"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
)

type adminPaymentMethodMarketPreviewRequest struct {
	Source          adminPluginMarketSourceRequest `json:"source"`
	Kind            string                         `json:"kind"`
	Name            string                         `json:"name"`
	Version         string                         `json:"version"`
	PaymentMethodID *uint                          `json:"payment_method_id"`
}

type adminPaymentMethodMarketImportRequest struct {
	Source             adminPluginMarketSourceRequest `json:"source"`
	Kind               string                         `json:"kind"`
	Name               string                         `json:"name"`
	Version            string                         `json:"version"`
	PaymentMethodID    *uint                          `json:"payment_method_id"`
	PaymentName        string                         `json:"payment_name"`
	PaymentDescription string                         `json:"payment_description"`
	Icon               string                         `json:"icon"`
	Entry              string                         `json:"entry"`
	Config             string                         `json:"config"`
	PollInterval       *int                           `json:"poll_interval"`
}

func (h *PaymentMethodHandler) ListMarketSources(c *gin.Context) {
	result, err := service.ListPaymentMethodMarketSources(h.db)
	if err != nil {
		h.respondPaymentMethodMarketError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *PaymentMethodHandler) ListMarketCatalog(c *gin.Context) {
	result, err := service.ListPaymentMethodMarketCatalog(h.db, map[string]interface{}{
		"source_id": c.Query("source_id"),
		"kind":      c.Query("kind"),
		"channel":   c.Query("channel"),
		"q":         c.Query("q"),
		"offset":    c.Query("offset"),
		"limit":     c.Query("limit"),
	})
	if err != nil {
		h.respondPaymentMethodMarketError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *PaymentMethodHandler) GetMarketArtifact(c *gin.Context) {
	result, err := service.GetPaymentMethodMarketArtifact(h.db, map[string]interface{}{
		"source_id": c.Query("source_id"),
		"kind":      c.Query("kind"),
		"name":      c.Param("name"),
	})
	if err != nil {
		h.respondPaymentMethodMarketError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *PaymentMethodHandler) paymentMethodMarketRuntime() *service.PluginHostRuntime {
	return service.NewPluginHostRuntime(h.db, config.GetConfig(), nil)
}

func (h *PaymentMethodHandler) respondPaymentMethodMarketError(c *gin.Context, err error) {
	var hostErr *service.PluginHostActionError
	if errors.As(err, &hostErr) {
		switch hostErr.Status {
		case http.StatusBadRequest:
			response.BadRequest(c, hostErr.Message)
		case http.StatusForbidden:
			response.Forbidden(c, hostErr.Message)
		case http.StatusNotFound:
			response.NotFound(c, hostErr.Message)
		case http.StatusConflict:
			response.Conflict(c, hostErr.Message)
		default:
			response.InternalServerError(c, hostErr.Message, err)
		}
		return
	}
	response.InternalServerError(c, "Market payment import failed", err)
}

func (h *PaymentMethodHandler) PreviewMarketPackage(c *gin.Context) {
	var req adminPaymentMethodMarketPreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid market preview request")
		return
	}

	kind := strings.ToLower(strings.TrimSpace(req.Kind))
	if kind == "" {
		kind = "payment_package"
	}
	if kind != "payment_package" {
		response.BadRequest(c, "market kind must be payment_package")
		return
	}

	var targetID uint
	if req.PaymentMethodID != nil {
		targetID = *req.PaymentMethodID
	}

	result, err := service.PreviewPaymentMethodMarketPackageWithSource(
		h.paymentMethodMarketRuntime(),
		buildAdminPluginMarketSource(req.Source),
		req.Name,
		req.Version,
		targetID,
	)
	if err != nil {
		h.respondPaymentMethodMarketError(c, err)
		return
	}

	logger.LogOperation(h.db, c, "payment_method_market_preview", "payment_method", nil, map[string]interface{}{
		"source_id":         strings.TrimSpace(req.Source.SourceID),
		"kind":              kind,
		"name":              strings.TrimSpace(req.Name),
		"version":           strings.TrimSpace(req.Version),
		"payment_method_id": targetID,
	})
	response.Success(c, result)
}

func (h *PaymentMethodHandler) ImportPackageFromMarket(c *gin.Context) {
	var req adminPaymentMethodMarketImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid market import request")
		return
	}
	adminID, _ := middleware.GetUserID(c)

	kind := strings.ToLower(strings.TrimSpace(req.Kind))
	if kind == "" {
		kind = "payment_package"
	}
	if kind != "payment_package" {
		response.BadRequest(c, "market kind must be payment_package")
		return
	}

	params := map[string]interface{}{}
	if req.PaymentMethodID != nil {
		params["payment_method_id"] = *req.PaymentMethodID
	}
	if value := strings.TrimSpace(req.PaymentName); value != "" {
		params["name"] = value
	}
	if value := strings.TrimSpace(req.PaymentDescription); value != "" {
		params["description"] = value
	}
	if value := strings.TrimSpace(req.Icon); value != "" {
		params["icon"] = value
	}
	if value := strings.TrimSpace(req.Entry); value != "" {
		params["entry"] = value
	}
	if value := strings.TrimSpace(req.Config); value != "" {
		params["config"] = value
	}
	if req.PollInterval != nil {
		params["poll_interval"] = *req.PollInterval
	}
	if h.pluginManager != nil {
		hookExecCtx := buildAdminHookExecutionContext(c, &adminID, map[string]string{
			"resource_type": "payment_market",
		})
		hookPayload := map[string]interface{}{
			"source":            "admin_api",
			"market_source_id":  strings.TrimSpace(req.Source.SourceID),
			"name":              strings.TrimSpace(req.Name),
			"version":           strings.TrimSpace(req.Version),
			"payment_method_id": req.PaymentMethodID,
		}
		for key, value := range params {
			hookPayload[key] = value
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "payment.market.install.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("payment.market.install.before hook execution failed: name=%s version=%s err=%v", req.Name, req.Version, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Market payment import rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if value, exists := hookResult.Payload["payment_method_id"]; exists {
					params["payment_method_id"] = value
				}
				if value, exists := hookResult.Payload["name"]; exists {
					params["name"] = parseStringFromAny(value)
				}
				if value, exists := hookResult.Payload["description"]; exists {
					params["description"] = parseStringFromAny(value)
				}
				if value, exists := hookResult.Payload["icon"]; exists {
					params["icon"] = parseStringFromAny(value)
				}
				if value, exists := hookResult.Payload["entry"]; exists {
					params["entry"] = parseStringFromAny(value)
				}
				if value, exists := hookResult.Payload["config"]; exists {
					params["config"] = parseStringFromAny(value)
				}
				if value, exists := hookResult.Payload["poll_interval"]; exists {
					params["poll_interval"] = parseIntFromAny(value, 0)
				}
			}
		}
	}

	result, err := service.ExecutePaymentMethodMarketPackageWithSource(
		h.paymentMethodMarketRuntime(),
		buildAdminPluginMarketSource(req.Source),
		req.Name,
		req.Version,
		params,
	)
	if err != nil {
		h.respondPaymentMethodMarketError(c, err)
		return
	}

	var methodID *uint
	if method, ok := result["item"].(*models.PaymentMethod); ok && method != nil && method.ID > 0 {
		methodID = &method.ID
	}
	action := "payment_method_market_import"
	if created, _ := result["created"].(bool); !created {
		action = "payment_method_market_update"
	}
	logger.LogOperation(h.db, c, action, "payment_method", methodID, map[string]interface{}{
		"source_id":         strings.TrimSpace(req.Source.SourceID),
		"kind":              kind,
		"name":              strings.TrimSpace(req.Name),
		"version":           strings.TrimSpace(req.Version),
		"payment_method_id": req.PaymentMethodID,
		"created":           result["created"],
	})
	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"source":            "admin_api",
			"market_source_id":  strings.TrimSpace(req.Source.SourceID),
			"name":              strings.TrimSpace(req.Name),
			"version":           strings.TrimSpace(req.Version),
			"payment_method_id": req.PaymentMethodID,
			"created":           result["created"],
			"imported_by":       adminID,
			"result":            result,
		}
		if methodID != nil {
			afterPayload["resolved_payment_method_id"] = *methodID
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, targetID *uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "payment.market.install.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				methodIDValue := uint(0)
				if targetID != nil {
					methodIDValue = *targetID
				}
				log.Printf("payment.market.install.after hook execution failed: payment_method_id=%d err=%v", methodIDValue, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, &adminID, map[string]string{
			"resource_type": "payment_market",
		})), afterPayload, methodID)
	}
	response.Success(c, result)
}
