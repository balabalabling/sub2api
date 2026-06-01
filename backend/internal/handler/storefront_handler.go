package handler

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type StorefrontHandler struct {
	paymentService *service.PaymentService
}

func NewStorefrontHandler(paymentService *service.PaymentService) *StorefrontHandler {
	return &StorefrontHandler{paymentService: paymentService}
}

func (h *StorefrontHandler) ListProducts(c *gin.Context) {
	products, err := h.paymentService.ListStoreProducts(c.Request.Context(), true)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, products)
}

func (h *StorefrontHandler) GetProduct(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid product id")
		return
	}
	product, err := h.paymentService.GetStoreProduct(c.Request.Context(), id, true)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, product)
}

type storefrontCreateOrderRequest struct {
	Email       string `json:"email" binding:"required"`
	ProductID   int64  `json:"product_id" binding:"required"`
	PaymentType string `json:"payment_type"`
	ReturnURL   string `json:"return_url"`
	IsMobile    *bool  `json:"is_mobile"`
}

func (h *StorefrontHandler) CreateOrder(c *gin.Context) {
	var req storefrontCreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	mobile := isMobile(c)
	if req.IsMobile != nil {
		mobile = *req.IsMobile
	}
	result, err := h.paymentService.CreateStorefrontOrder(c.Request.Context(), service.StorefrontCreateOrderInput{
		Email:       req.Email,
		ProductID:   req.ProductID,
		PaymentType: req.PaymentType,
		ClientIP:    c.ClientIP(),
		IsMobile:    mobile,
		SrcHost:     c.Request.Host,
		SrcURL:      c.Request.Referer(),
		ReturnURL:   req.ReturnURL,
		Locale:      c.GetHeader("Accept-Language"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

type storefrontSendCodeRequest struct {
	Email string `json:"email" binding:"required"`
}

func (h *StorefrontHandler) SendQueryCode(c *gin.Context) {
	var req storefrontSendCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	if err := h.paymentService.SendStoreQueryCode(c.Request.Context(), req.Email, c.GetHeader("Accept-Language")); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "sent"})
}

type storefrontVerifyCodeRequest struct {
	Email string `json:"email" binding:"required"`
	Code  string `json:"code" binding:"required"`
}

func (h *StorefrontHandler) VerifyQueryCode(c *gin.Context) {
	var req storefrontVerifyCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	token, err := h.paymentService.VerifyStoreQueryCode(c.Request.Context(), req.Email, req.Code)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"query_token": token})
}

func (h *StorefrontHandler) QueryUsage(c *gin.Context) {
	key := strings.TrimSpace(c.Query("key"))
	if key != "" {
		items, err := h.paymentService.ListStoreUsageByKey(c.Request.Context(), key)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		response.Success(c, gin.H{"items": items})
		return
	}
	email := strings.TrimSpace(c.Query("email"))
	token := strings.TrimSpace(c.Query("query_token"))
	items, err := h.paymentService.ListStoreUsageByEmail(c.Request.Context(), email, token)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}
