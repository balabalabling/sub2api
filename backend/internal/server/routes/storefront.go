package routes

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func RegisterStorefrontRoutes(v1 *gin.RouterGroup, h *handler.Handlers, redisClient *redis.Client) {
	rateLimiter := middleware.NewRateLimiter(redisClient)

	storefront := v1.Group("/storefront")
	{
		storefront.GET("/products", h.Storefront.ListProducts)
		storefront.GET("/products/:id", h.Storefront.GetProduct)
		storefront.POST("/orders", rateLimiter.Limit("storefront-orders", 20, time.Minute), h.Storefront.CreateOrder)
		storefront.POST("/query/send-code", rateLimiter.Limit("storefront-query-code", 5, time.Minute), h.Storefront.SendQueryCode)
		storefront.POST("/query/verify-code", rateLimiter.Limit("storefront-query-verify", 10, time.Minute), h.Storefront.VerifyQueryCode)
		storefront.GET("/usage", rateLimiter.Limit("storefront-usage", 60, time.Minute), h.Storefront.QueryUsage)
	}
}
