package api

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/keith/goedgeinfer/internal/config"
	"github.com/keith/goedgeinfer/internal/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// SetupRoutes configures all the API routes
func SetupRoutes(r *gin.Engine, api *API, apiKey string, jwtCfg config.JWTConfig, logger *zap.Logger) {
	// Add Prometheus metrics endpoint (no rate limiting, no auth)
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Health check endpoint (no rate limiting, no auth)
	r.GET("/health", api.HealthCheck)

	// API key auth for all other endpoints
	r.Use(middleware.APIKeyAuthMiddleware(apiKey))

	// API versioning
	v1 := r.Group("/api/v1")
	v1.Use(
		middleware.RateLimitMiddleware(10.0, 20), // 10 requests per second with burst of 20
		middleware.RequestIDMiddleware(),        // Add request ID for tracing
	)

	// Public endpoints (no auth required)
	public := v1.Group("/public")
	{
		public.GET("/health", api.HealthCheck)
	}

	// Model endpoints (API key required)
	models := v1.Group("/models")
	{
		// List all loaded models
		models.GET("", api.ListModels)
		
		// Load a new model
		models.POST("", api.LoadModel)
		
		// Model-specific operations
		modelGroup := models.Group("/:model_id")
		{
			// Get model info
			modelGroup.GET("", api.GetModelInfo)
			
			// Unload a model
			modelGroup.DELETE("", api.UnloadModel)
			
			// Versioned model operations
			versionGroup := modelGroup.Group("/versions/:version")
			{
				// Get specific version info
				versionGroup.GET("", api.GetModelInfo)
				
				// Unload specific version
				versionGroup.DELETE("", api.UnloadModel)
			}
		}
	}

	// Inference endpoints (API key required)
	infer := v1.Group("/infer")
	{
		// Predict with model ID in path (supports version in request body)
		infer.POST("/:model_id", api.Predict)
		
		// Versioned prediction
		infer.POST("/:model_id/versions/:version", api.Predict)
		
		// Batch prediction
		infer.POST("/batch/:model_id", api.BatchPredict)
		infer.POST("/batch/:model_id/versions/:version", api.BatchPredict)
	}

	// Management endpoints (JWT required)
	if jwtCfg.Enabled {
		mgmt := v1.Group("/mgmt")
		mgmt.Use(middleware.JWTMiddleware(middleware.JWTConfig{
			Secret:       jwtCfg.Secret,
			Algorithm:    jwtCfg.Algorithm,
			Audience:     jwtCfg.Audience,
			Issuer:       jwtCfg.Issuer,
			RequireRole:  jwtCfg.RequireRole,
			RequireScope: jwtCfg.RequireScope,
		}))
		{
			// Remote model management
			remote := mgmt.Group("/remote")
			{
				remote.GET("/models", api.ListRemoteModels)
				remote.POST("/cleanup", api.CleanupModelCache)
				remote.POST("/delete", api.DeleteRemoteModel)
				remote.POST("/upload", api.UploadRemoteModel)
			}

			// System management
			system := mgmt.Group("/system")
			{
				system.POST("/reload", api.Reload)
				system.GET("/config", func(c *gin.Context) {
					// Only admin can view
					if claims, exists := c.Get("jwt_claims"); !exists || claims.(map[string]interface{})["role"] != "admin" {
						c.AbortWithStatusJSON(403, ErrorResponse{Error: "admin only"})
						return
					}
					c.JSON(200, gin.H{
						"jwt":     jwtCfg,
						"version": getVersionInfo(),
					})
				})
			}
		}
	}

	// Log registered routes
	logRoutes(r, logger)
}

// logRoutes logs all registered routes for debugging
func logRoutes(r *gin.Engine, logger *zap.Logger) {
	routes := r.Routes()
	logger.Info("Registered routes:")
	for _, route := range routes {
		logger.Info("route",
			zap.String("method", route.Method),
			zap.String("path", route.Path),
			zap.String("handler", route.Handler),
		)
	}
}

// getVersionInfo returns version information about the API
func getVersionInfo() map[string]string {
	return map[string]string{
		"version":   "1.0.0",
		"build":     "dev",
		"buildTime": time.Now().UTC().Format(time.RFC3339),
	}
}
