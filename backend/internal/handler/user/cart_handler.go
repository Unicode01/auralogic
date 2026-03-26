package user

import (
	"errors"
	"log"
	"strconv"
	"strings"

	"auralogic/internal/middleware"
	"auralogic/internal/pkg/bizerr"
	"auralogic/internal/pkg/response"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
)

type CartHandler struct {
	cartService   *service.CartService
	pluginManager *service.PluginManagerService
}

func NewCartHandler(cartService *service.CartService, pluginManager *service.PluginManagerService) *CartHandler {
	return &CartHandler{
		cartService:   cartService,
		pluginManager: pluginManager,
	}
}

// GetCart 获取购物车
func (h *CartHandler) GetCart(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	items, err := h.cartService.GetCart(userID)
	if err != nil {
		response.InternalError(c, "Failed to get cart")
		return
	}

	// 计算总价
	var totalPrice int64
	var totalQuantity int
	for _, item := range items {
		if item.IsAvailable {
			totalPrice += item.Price * int64(item.Quantity)
		}
		totalQuantity += item.Quantity
	}

	response.Success(c, gin.H{
		"items":             items,
		"total_price_minor": totalPrice,
		"total_quantity":    totalQuantity,
		"item_count":        len(items),
	})
}

// AddToCartRequest 添加到购物车请求
type AddToCartRequest struct {
	ProductID  uint              `json:"product_id" binding:"required"`
	Quantity   int               `json:"quantity" binding:"required,min=1"`
	Attributes map[string]string `json:"attributes"`
}

// AddToCart 添加商品到购物车
func (h *CartHandler) AddToCart(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	var req AddToCartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	hookExecCtx := buildUserHookExecutionContext(c, userID, map[string]string{
		"hook_resource": "cart",
		"hook_source":   "user_api",
	})
	if h.pluginManager != nil {
		originalReq := req
		hookPayload, payloadErr := userHookStructToPayload(req)
		if payloadErr != nil {
			log.Printf("cart.add.before payload build failed: user=%d err=%v", userID, payloadErr)
		} else {
			hookPayload["user_id"] = userID
			hookPayload["source"] = "user_api"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "cart.add.before",
				Payload: hookPayload,
			}, hookExecCtx)
			if hookErr != nil {
				log.Printf("cart.add.before hook execution failed: user=%d product=%d err=%v", userID, req.ProductID, hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "Add to cart rejected by plugin"
					}
					response.BadRequest(c, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeUserHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("cart.add.before payload apply failed, fallback to original request: user=%d err=%v", userID, mergeErr)
						req = originalReq
					}
				}
			}
		}
	}
	if req.ProductID == 0 || req.Quantity <= 0 {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	item, err := h.cartService.AddToCart(userID, service.AddToCartRequest{
		ProductID:  req.ProductID,
		Quantity:   req.Quantity,
		Attributes: req.Attributes,
	})
	if err != nil {
		var bizErr *bizerr.Error
		if errors.As(err, &bizErr) {
			response.BizError(c, bizErr.Message, bizErr.Key, bizErr.Params)
			return
		}
		response.HandleError(c, "Failed to add to cart", err)
		return
	}
	if h.pluginManager != nil && item != nil {
		afterPayload := map[string]interface{}{
			"user_id":      userID,
			"item_id":      item.ID,
			"product_id":   item.ProductID,
			"quantity":     item.Quantity,
			"attributes":   item.Attributes,
			"price_minor":  item.Price,
			"product_name": item.Name,
			"source":       "user_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, itemID uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "cart.add.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("cart.add.after hook execution failed: user=%d item=%d err=%v", userID, itemID, hookErr)
			}
		}(cloneUserHookExecutionContext(hookExecCtx), afterPayload, item.ID)
	}

	response.Success(c, gin.H{
		"item":    item,
		"message": "Added to cart",
	})
}

// UpdateQuantityRequest 更新数量请求
type UpdateQuantityRequest struct {
	Quantity int `json:"quantity" binding:"required,min=1"`
}

// UpdateQuantity 更新购物车项数量
func (h *CartHandler) UpdateQuantity(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	itemID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid cart item ID")
		return
	}

	var req UpdateQuantityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	hookExecCtx := buildUserHookExecutionContext(c, userID, map[string]string{
		"hook_resource": "cart",
		"hook_source":   "user_api",
	})
	if h.pluginManager != nil {
		hookReq := struct {
			Quantity int `json:"quantity"`
		}{
			Quantity: req.Quantity,
		}
		hookPayload, payloadErr := userHookStructToPayload(hookReq)
		if payloadErr != nil {
			log.Printf("cart.update.before payload build failed: user=%d item=%d err=%v", userID, itemID, payloadErr)
		} else {
			hookPayload["user_id"] = userID
			hookPayload["item_id"] = uint(itemID)
			hookPayload["source"] = "user_api"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "cart.update.before",
				Payload: hookPayload,
			}, hookExecCtx)
			if hookErr != nil {
				log.Printf("cart.update.before hook execution failed: user=%d item=%d err=%v", userID, itemID, hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "Cart update rejected by plugin"
					}
					response.BadRequest(c, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeUserHookStructPatch(&hookReq, hookResult.Payload); mergeErr != nil {
						log.Printf("cart.update.before payload apply failed, keeping original request: user=%d item=%d err=%v", userID, itemID, mergeErr)
					} else {
						req.Quantity = hookReq.Quantity
					}
				}
			}
		}
	}
	if req.Quantity <= 0 {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	item, err := h.cartService.UpdateQuantity(userID, uint(itemID), req.Quantity)
	if err != nil {
		var bizErr *bizerr.Error
		if errors.As(err, &bizErr) {
			response.BizError(c, bizErr.Message, bizErr.Key, bizErr.Params)
			return
		}
		response.HandleError(c, "Failed to update quantity", err)
		return
	}
	if h.pluginManager != nil && item != nil {
		afterPayload := map[string]interface{}{
			"user_id":      userID,
			"item_id":      item.ID,
			"product_id":   item.ProductID,
			"quantity":     item.Quantity,
			"attributes":   item.Attributes,
			"price_minor":  item.Price,
			"product_name": item.Name,
			"source":       "user_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, cartItemID uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "cart.update.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("cart.update.after hook execution failed: user=%d item=%d err=%v", userID, cartItemID, hookErr)
			}
		}(cloneUserHookExecutionContext(hookExecCtx), afterPayload, item.ID)
	}

	response.Success(c, gin.H{
		"item":    item,
		"message": "Quantity updated",
	})
}

// RemoveFromCart 从购物车移除商品
func (h *CartHandler) RemoveFromCart(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	itemID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid cart item ID")
		return
	}
	var removedItemID uint = uint(itemID)
	var removedProductID uint
	var removedName string
	var removedQuantity int
	if h.pluginManager != nil {
		if items, loadErr := h.cartService.GetCart(userID); loadErr == nil {
			for _, item := range items {
				if item.ID == removedItemID {
					removedProductID = item.ProductID
					removedName = item.Name
					removedQuantity = item.Quantity
					break
				}
			}
		}
	}

	if err := h.cartService.RemoveFromCart(userID, uint(itemID)); err != nil {
		var bizErr *bizerr.Error
		if errors.As(err, &bizErr) {
			response.BizError(c, bizErr.Message, bizErr.Key, bizErr.Params)
			return
		}
		response.HandleError(c, "Failed to remove from cart", err)
		return
	}
	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"user_id":      userID,
			"item_id":      removedItemID,
			"product_id":   removedProductID,
			"product_name": removedName,
			"quantity":     removedQuantity,
			"source":       "user_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, cartItemID uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "cart.remove.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("cart.remove.after hook execution failed: user=%d item=%d err=%v", userID, cartItemID, hookErr)
			}
		}(cloneUserHookExecutionContext(buildUserHookExecutionContext(c, userID, map[string]string{
			"hook_resource": "cart",
			"hook_source":   "user_api",
		})), afterPayload, removedItemID)
	}

	response.Success(c, gin.H{
		"message": "Removed from cart",
	})
}

// ClearCart 清空购物车
func (h *CartHandler) ClearCart(c *gin.Context) {
	userID := middleware.MustGetUserID(c)
	clearedCount := 0
	clearedQuantity := 0
	if h.pluginManager != nil {
		if items, loadErr := h.cartService.GetCart(userID); loadErr == nil {
			clearedCount = len(items)
			for _, item := range items {
				clearedQuantity += item.Quantity
			}
		}
	}

	if err := h.cartService.ClearCart(userID); err != nil {
		response.InternalError(c, "Failed to clear cart")
		return
	}
	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"user_id":       userID,
			"cleared_count": clearedCount,
			"cleared_items": clearedCount,
			"cleared_units": clearedQuantity,
			"source":        "user_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "cart.clear.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("cart.clear.after hook execution failed: user=%d err=%v", userID, hookErr)
			}
		}(cloneUserHookExecutionContext(buildUserHookExecutionContext(c, userID, map[string]string{
			"hook_resource": "cart",
			"hook_source":   "user_api",
		})), afterPayload)
	}

	response.Success(c, gin.H{
		"message": "Cart cleared",
	})
}

// GetCartCount 获取购物车商品数量
func (h *CartHandler) GetCartCount(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	count, err := h.cartService.GetCartCount(userID)
	if err != nil {
		response.InternalError(c, "Failed to get cart count")
		return
	}

	response.Success(c, gin.H{
		"count": count,
	})
}
