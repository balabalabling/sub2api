package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type StoreHandler struct {
	paymentService *service.PaymentService
}

func NewStoreHandler(paymentService *service.PaymentService) *StoreHandler {
	return &StoreHandler{paymentService: paymentService}
}

func (h *StoreHandler) ListProducts(c *gin.Context) {
	products, err := h.paymentService.ListStoreProducts(c.Request.Context(), false)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, products)
}

func (h *StoreHandler) CreateProduct(c *gin.Context) {
	var req service.StoreProductInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	product, err := h.paymentService.CreateStoreProduct(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, product)
}

func (h *StoreHandler) UpdateProduct(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid product id")
		return
	}
	var req service.StoreProductInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	product, err := h.paymentService.UpdateStoreProduct(c.Request.Context(), id, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, product)
}

func (h *StoreHandler) DeleteProduct(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid product id")
		return
	}
	if err := h.paymentService.DeleteStoreProduct(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "deleted"})
}
