package user

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"auralogic/internal/middleware"
	"auralogic/internal/pkg/response"
	"auralogic/internal/service"
)

type CartHandler struct {
	cartService *service.CartService
}

func NewCartHandler(cartService *service.CartService) *CartHandler {
	return &CartHandler{
		cartService: cartService,
	}
}

// GetCart 获取购物车
func (h *CartHandler) GetCart(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	items, err := h.cartService.GetCart(userID)
	if err != nil {
		response.InternalError(c, "获取购物车失败: "+err.Error())
		return
	}

	// 计算总价
	var totalPrice float64
	var totalQuantity int
	for _, item := range items {
		if item.IsAvailable {
			totalPrice += item.Price * float64(item.Quantity)
		}
		totalQuantity += item.Quantity
	}

	response.Success(c, gin.H{
		"items":          items,
		"total_price":    totalPrice,
		"total_quantity": totalQuantity,
		"item_count":     len(items),
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
		response.BadRequest(c, "请求参数无效")
		return
	}

	item, err := h.cartService.AddToCart(userID, service.AddToCartRequest{
		ProductID:  req.ProductID,
		Quantity:   req.Quantity,
		Attributes: req.Attributes,
	})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, gin.H{
		"item":    item,
		"message": "已添加到购物车",
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
		response.BadRequest(c, "无效的购物车项ID")
		return
	}

	var req UpdateQuantityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数无效")
		return
	}

	item, err := h.cartService.UpdateQuantity(userID, uint(itemID), req.Quantity)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, gin.H{
		"item":    item,
		"message": "数量已更新",
	})
}

// RemoveFromCart 从购物车移除商品
func (h *CartHandler) RemoveFromCart(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	itemID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的购物车项ID")
		return
	}

	if err := h.cartService.RemoveFromCart(userID, uint(itemID)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, gin.H{
		"message": "已从购物车移除",
	})
}

// ClearCart 清空购物车
func (h *CartHandler) ClearCart(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	if err := h.cartService.ClearCart(userID); err != nil {
		response.InternalError(c, "清空购物车失败")
		return
	}

	response.Success(c, gin.H{
		"message": "购物车已清空",
	})
}

// GetCartCount 获取购物车商品数量
func (h *CartHandler) GetCartCount(c *gin.Context) {
	userID := middleware.MustGetUserID(c)

	count, err := h.cartService.GetCartCount(userID)
	if err != nil {
		response.InternalError(c, "获取购物车数量失败")
		return
	}

	response.Success(c, gin.H{
		"count": count,
	})
}
